package computemapping

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"go.chromium.org/luci/common/data/stringset"
	"go.chromium.org/luci/common/logging"

	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/git"
	"go.chromium.org/infra/tools/dirmd"
	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

const (
	// gitDateLayout is a layout string for git's `--shallow-since` argument.
	gitDateLayout = "Jan 02 2006"
)

// MappingInfo groups a computed Mapping and affected files for a set
// of ChangeRevs in a project.
type MappingInfo struct {
	Mapping       *dirmd.Mapping
	AffectedFiles []string
}

// WorkdirCreation is a function signature that returns a path to a workdir,
// a cleanup function, and an error if one occurred.
type WorkdirCreation func() (string, func() error, error)

// changeRevsContainDirmd returns true if any file in changeRevs is a
// DIR_METADATA.
func changeRevsContainDirmd(changeRevs []*gerrit.ChangeRev) bool {
	for _, changeRev := range changeRevs {
		for _, file := range changeRev.Files {
			// Use path because the filenames are coming from Gerrit, so are
			// slash-separated (not whatever the local OS separator is).
			if path.Base(file) == "DIR_METADATA" {
				return true
			}
		}
	}

	return false
}

// computeShallowSince finds the earliest creation time in changeRevs and returns
// a time buffer before this earliest creation time. This is meant to be used
// as the `--shallow-since` argument to git clone or fetch.
func computeShallowSince(ctx context.Context, changeRevs []*gerrit.ChangeRev, buffer time.Duration) time.Time {
	minTime := changeRevs[0].ChangeCreated.AsTime()
	for _, changeRev := range changeRevs {
		if changeRev.ChangeCreated.AsTime().Before(minTime) {
			minTime = changeRev.ChangeCreated.AsTime()
		}
	}

	logging.Debugf(ctx, "found change rev with min time: %q", &minTime)

	shallowSince := minTime.Add(-buffer).Truncate(time.Hour * 24)

	logging.Debugf(ctx, "after subtracting buffer and truncating, using shallow since: %q", shallowSince)

	return shallowSince
}

// mergeChangeRevs merges changeRevs to dir.
//
// changeRevs must all have the same project and branch. For each changeRev, if
// `git merge` fails `git cherry-pick` will be attempted as a fallback.
func mergeChangeRevs(
	ctx context.Context,
	dir string,
	changeRevs []*gerrit.ChangeRev,
	cloneDepth time.Duration,
) error {
	for i, changeRev := range changeRevs {
		if i > 0 && (changeRev.Project != changeRevs[0].Project || changeRev.Branch != changeRevs[0].Branch) {
			// Change revs are sorted by project and branch in the callers.
			panic(
				"all changeRevs passed to checkoutChangeRevs must have the same Project and Branch",
			)
		}
	}

	// All changeRevs must have the same Host, Project, and Branch, as checked above.
	googlesourceHost := strings.Replace(changeRevs[0].Host, "-review", "", 1)
	remote := fmt.Sprintf("https://%s/%s", googlesourceHost, changeRevs[0].Project)
	branch := strings.Replace(changeRevs[0].Branch, "refs/heads/", "", 1)

	// If none of the ChangeRevs contain a DIR_METADATA, there is no need to
	// merge / cherry-pick the changes, since we only care about changed
	// DIR_METADATAS. Just clone the repo with depth=1 for speed.
	if !changeRevsContainDirmd(changeRevs) {
		logging.Debugf(ctx, "change revs for repo %q, branch %q don't affect DIR_METADATA files, cloning with depth=1", remote, branch)
		return git.Clone(remote, dir, git.NoTags(), git.Branch(branch), git.Depth(1))
	}

	shallowSinceTime := computeShallowSince(ctx, changeRevs, cloneDepth)
	logging.Debugf(ctx, "change revs for repo %q, branch %q do affect DIR_METADATA files, cloning with shallow-since %q", remote, branch, shallowSinceTime)

	// If the --shallow-since clone fails, fall back to doing a full clone. If
	// the clone is done without --shallow-since, the later fetches must also
	// be done without --shallow-since, so track this case in
	// shallowSinceFailed.
	shallowSinceFailed := false
	if err := git.Clone(remote, dir, git.NoTags(), git.Branch(branch), git.ShallowSince(shallowSinceTime.Format(gitDateLayout))); err != nil {
		logging.Errorf(ctx, "cloning with with shallow-since failed, attempting full clone")
		shallowSinceFailed = true
		if fullCloneErr := git.Clone(remote, dir, git.NoTags(), git.Branch(branch)); fullCloneErr != nil {
			return fullCloneErr
		}
	}

	for _, changeRev := range changeRevs {
		// For each changeRev, first attempt to merge the change, and if that
		// fails attempt to cherry pick the change instead. This behavior should
		// be kept consistent with how CQ builders apply changes.
		logging.Debugf(ctx, "fetching ref %q from repo %q", changeRev.Ref, remote)

		if shallowSinceFailed {
			logging.Warningf(ctx, "shallow-since failed on clone, now a full fetch is required")
			if err := git.Fetch(dir, remote, changeRev.Ref, git.NoTags()); err != nil {
				return err
			}
		} else if err := git.Fetch(dir, remote, changeRev.Ref, git.NoTags(), git.ShallowSince(shallowSinceTime.Format(gitDateLayout))); err != nil {
			return err
		}

		if mergeErr := git.Merge(ctx, dir, "FETCH_HEAD"); mergeErr != nil {
			logging.Warningf(
				ctx,
				"failed to merge change rev %q (got error %q), aborting merge and attempting cherry-pick instead",
				changeRev,
				mergeErr,
			)
			if abortErr := git.MergeAbort(ctx, dir); abortErr != nil {
				return abortErr
			}

			if cherryPickErr := git.CherryPick(ctx, dir, "FETCH_HEAD"); cherryPickErr != nil {
				return cherryPickErr
			}
		}
	}

	return nil
}

// computeMappingForChangeRevs checks out a project with changeRevs applied and
// computes the Mapping.
//
// changeRevs must all have the same project.
func computeMappingForChangeRevs(
	ctx context.Context,
	changeRevs []*gerrit.ChangeRev,
	workdirFn WorkdirCreation,
	cloneDepth time.Duration,
) (mapping *dirmd.Mapping, err error) {
	workdir, cleanup, err := workdirFn()
	if err != nil {
		return nil, err
	}

	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			err = cleanupErr
		}
	}()

	if err = mergeChangeRevs(ctx, workdir, changeRevs, cloneDepth); err != nil {
		return nil, err
	}

	mapping, err = dirmd.ReadMapping(ctx, dirmdpb.MappingForm_COMPUTED, true, workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to read DIR_METADATA for change revs %q: %w", changeRevs, err)
	}

	if mapping == nil {
		return nil, fmt.Errorf("got nil mapping for change revs %q", changeRevs)
	}

	return mapping, nil
}

// computeProjectMappingInfos calculates a projectMappingInfo for each project
// and branch in changeRevs.
func ProjectInfos(
	ctx context.Context,
	changeRevs []*gerrit.ChangeRev,
	workdirFn WorkdirCreation,
	cloneDepth time.Duration,
) ([]*MappingInfo, error) {
	projectToBranchToChangeRevs := make(map[string]map[string][]*gerrit.ChangeRev)
	projectToBranchToAffectedFiles := make(map[string]map[string]stringset.Set)

	for _, changeRev := range changeRevs {
		project := changeRev.Project
		branch := changeRev.Branch

		// Create a slice of ChangeRevs for project and branch, if it does not
		// already exist, then add changeRev.
		if _, found := projectToBranchToChangeRevs[project]; !found {
			projectToBranchToChangeRevs[project] = make(map[string][]*gerrit.ChangeRev)
		}

		if _, found := projectToBranchToChangeRevs[project][branch]; !found {
			projectToBranchToChangeRevs[project][branch] = make([]*gerrit.ChangeRev, 0)
		}

		projectToBranchToChangeRevs[project][branch] = append(
			projectToBranchToChangeRevs[project][branch], changeRev,
		)

		// Create a stringset.Set for project and branch, if it does not already
		// exist, then add changeRev.Files.
		if _, found := projectToBranchToAffectedFiles[project]; !found {
			projectToBranchToAffectedFiles[project] = make(map[string]stringset.Set)
		}

		if _, found := projectToBranchToAffectedFiles[project][branch]; !found {
			projectToBranchToAffectedFiles[project][branch] = stringset.New(0)
		}

		projectToBranchToAffectedFiles[project][branch].AddAll(changeRev.Files)
	}

	projectMappingInfos := make([]*MappingInfo, 0)

	// Use a sorted list of projects from projectToBranchToChangeRevs, so
	// iteration order is deterministic.
	projects := make([]string, 0, len(projectToBranchToChangeRevs))
	for project := range projectToBranchToChangeRevs {
		projects = append(projects, project)
	}

	sort.Strings(projects)

	for _, project := range projects {
		// Use a sorted list of branches from branchToChangeRevs, so iteration
		// order is deterministic.
		branchToChangeRevs := projectToBranchToChangeRevs[project]
		branches := make([]string, 0, len(branchToChangeRevs))
		for branch := range branchToChangeRevs {
			branches = append(branches, branch)
		}

		sort.Strings(branches)

		for _, branch := range branches {
			logging.Infof(ctx, "computing metadata for project %q, branch %q", project, branch)

			changeRevsForBranch := branchToChangeRevs[branch]

			mapping, err := computeMappingForChangeRevs(ctx, changeRevsForBranch, workdirFn, cloneDepth)
			if err != nil {
				return nil, err
			}

			projectMappingInfos = append(projectMappingInfos, &MappingInfo{
				AffectedFiles: projectToBranchToAffectedFiles[project][branch].ToSlice(),
				Mapping:       mapping,
			})
		}
	}

	return projectMappingInfos, nil
}
