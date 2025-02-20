// package migrationstatus summarizes the status of projects being migrated to
// distributed test config.
package migrationstatus

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/data/stringset"
	cvpb "go.chromium.org/luci/cv/api/config/v2"

	"go.chromium.org/infra/cros/internal/repo"
)

// names of relevant ConfigGroups and Builders in the CV config.
const (
	totConfigGroupName          = "ToT"
	cqOrchestratorCvBuilderName = "chromeos/cq/cq-orchestrator"
)

// orchestratorNames is a list of builder names with ProjectMigrationStatus
// input properties.
var orchestratorNames = []string{"cq-orchestrator", "staging-cq-orchestrator"}

// cqOrchProperties is used to parse the JSON input properties of orchestrators,
// must be kept in sync with https://source.chromium.org/chromium/chromiumos/infra/recipes/+/main:recipe_modules/cros_test_plan_v2/cros_test_plan_v2.proto.
type cqOrchProperties struct {
	CrosTestPlanV2 struct {
		MigrationConfigs []struct {
			Project          string
			ProjectBlocklist []string `json:"project_blocklist"`
		} `json:"migration_configs"`
	} `json:"$chromeos/cros_test_plan_v2"`
}

// getBuilderFromBuildbucketConfig finds builderName in bbCfg, returning an
// error if the builder is not found.
func getBuilderFromBuildbucketConfig(bbCfg *bbpb.BuildbucketCfg, builderName string) (*bbpb.BuilderConfig, error) {
	for _, bucket := range bbCfg.GetBuckets() {
		for _, builder := range bucket.GetSwarming().GetBuilders() {
			if builder.GetName() == builderName {
				return builder, nil
			}
		}
	}

	return nil, fmt.Errorf("no builder named %q found", builderName)
}

// getConfigGroup finds the ConfigGroup with name configGroup in cvConfig,
// returning an error if the ConfigGroup is not found.
func getConfigGroup(cvConfig *cvpb.Config, configGroup string) (*cvpb.ConfigGroup, error) {
	for _, group := range cvConfig.GetConfigGroups() {
		if group.GetName() == configGroup {
			return group, nil
		}
	}

	return nil, fmt.Errorf("ConfigGroup %q not found", configGroup)
}

// getProjectsFromConfigGroup gathers all the project names in configGroup and
// returns them as a set.
func getProjectsFromConfigGroup(configGroup *cvpb.ConfigGroup) stringset.Set {
	result := stringset.New(0)
	for _, gerrit := range configGroup.GetGerrit() {
		for _, project := range gerrit.GetProjects() {
			result.Add(project.GetName())
		}
	}

	return result
}

// getBuilderFromConfigGroup finds the Verifiers_Tryjob_Builder with
// name builderName in cvConfig, returning an error if the
// Verifiers_Tryjob_Builder is not found.
func getBuilderFromConfigGroup(cvConfig *cvpb.ConfigGroup, builderName string) (*cvpb.Verifiers_Tryjob_Builder, error) {
	for _, builder := range cvConfig.GetVerifiers().GetTryjob().GetBuilders() {
		if builder.GetName() == builderName {
			return builder, nil
		}
	}

	return nil, fmt.Errorf("Builder %q not found", builderName)
}

// projectIncludedForBuilder returns whether projectName is included by
// builder's LocationFilters. Only exclude LocationFilters are supported, and
// LocationFilters with project regexp .* are ignored (this function is tailored
// for the current CQ orchestrator CV config).
func projectIncludedForBuilder(builder *cvpb.Verifiers_Tryjob_Builder, projectName string) (bool, error) {
	for _, locationFilter := range builder.GetLocationFilters() {
		if !locationFilter.GetExclude() {
			return false, fmt.Errorf("only exclude LocationFilters currently supported, got %v", locationFilter)
		}

		if locationFilter.GetGerritProjectRegexp() == ".*" {
			continue
		}

		matched, err := regexp.Match(locationFilter.GetGerritProjectRegexp(), []byte(projectName))
		if err != nil {
			return false, err
		}

		if matched {
			return false, nil
		}
	}

	return true, nil
}

// MigrationStatus describes the distributed test planning status of a project.
type MigrationStatus struct {
	// The name of the builder that has a migration configured, usually an
	// orchestrator builder.
	BuilderName string

	// The name of the project.
	ProjectName string

	// The path the project is checked out to.
	ProjectPath string

	// Whether the project matches a MigrationConfig in the builder.
	MatchesMigrationConfig bool

	// Whether the project is included in the "ToT" CQ group.
	IncludedByToT bool

	// Whether the project is included by the builder's location filters.
	IncludedByBuilder bool
}

// Compute returns a MigrationStatus for each project in manifest and builder in
// orchestratorNames.
func Compute(
	ctx context.Context,
	manifest *repo.Manifest,
	bbCfg *bbpb.BuildbucketCfg,
	cvConfig *cvpb.Config,
) ([]*MigrationStatus, error) {
	totConfigGroup, err := getConfigGroup(cvConfig, totConfigGroupName)
	if err != nil {
		return nil, err
	}

	cqOrchCvConfig, err := getBuilderFromConfigGroup(totConfigGroup, cqOrchestratorCvBuilderName)
	if err != nil {
		return nil, err
	}

	totCvProjects := getProjectsFromConfigGroup(totConfigGroup)

	migrationStatuses := make([]*MigrationStatus, 0)

	for _, builderName := range orchestratorNames {
		builderConfig, err := getBuilderFromBuildbucketConfig(bbCfg, builderName)
		if err != nil {
			return nil, err
		}

		builderProps := &cqOrchProperties{}
		if err := json.Unmarshal([]byte(builderConfig.GetProperties()), builderProps); err != nil {
			return nil, fmt.Errorf("error parsing properties for builder %q: %w", builderName, err)
		}

		for _, project := range manifest.Projects {
			includedByCqOrch, err := projectIncludedForBuilder(cqOrchCvConfig, project.Name)
			if err != nil {
				return nil, err
			}

			matchesMigrationConfig := false

			for _, migrationConfig := range builderProps.CrosTestPlanV2.MigrationConfigs {
				// Empty project name in the MigrationConfig will regexp match
				// all strings, and is unexpected.
				if migrationConfig.Project == "" {
					return nil, fmt.Errorf("unexpected MigrationConfig with empty project: %q", migrationConfig)
				}

				matched, err := regexp.Match(migrationConfig.Project, []byte(project.Name))
				if err != nil {
					return nil, err
				}

				for _, blockedProject := range migrationConfig.ProjectBlocklist {
					matchedBlocklist, err := regexp.Match(blockedProject, []byte(project.Name))
					if err != nil {
						return nil, err
					}

					if matchedBlocklist {
						matched = false
						break
					}
				}

				if matched {
					matchesMigrationConfig = true
					break
				}
			}

			migrationStatuses = append(migrationStatuses, &MigrationStatus{
				BuilderName:            builderName,
				ProjectName:            project.Name,
				ProjectPath:            project.Path,
				MatchesMigrationConfig: matchesMigrationConfig,
				IncludedByToT:          totCvProjects.Has(project.Name),
				IncludedByBuilder:      includedByCqOrch,
			})
		}
	}

	return migrationStatuses, nil
}

// TextSummary returns a string summarizing migrationStatuses. If
// projectsToCheck is non-empty, the summary contains whether each specific
// project is migrated.
func TextSummary(
	ctx context.Context,
	migrationStatuses []*MigrationStatus,
	projectsToCheck []string,
) (string, error) {
	var summaryBuilder strings.Builder

	for _, builderName := range orchestratorNames {
		// Total # of projects for the builder, excluding projects not in the
		// ToT CQ group or project excluded by the builder.
		totalProjects := 0
		projectsMigrated := 0
		// Map from project name to MigrationStatus, to check each of
		// projectsToCheck later. Includes projects even if they are not in the
		// ToT CQ group or are excluded by the builder.
		projectToMigrationStatus := make(map[string]*MigrationStatus)

		for _, status := range migrationStatuses {
			// If the migration status isn't for builderName, skip.
			if status.BuilderName != builderName {
				continue
			}

			projectToMigrationStatus[status.ProjectName] = status

			if !status.IncludedByToT || !status.IncludedByBuilder {
				continue
			}

			totalProjects += 1
			if status.MatchesMigrationConfig {
				projectsMigrated += 1
			}
		}

		for _, project := range projectsToCheck {
			status, found := projectToMigrationStatus[project]
			if !found {
				return "", fmt.Errorf("no status found for project %q", project)
			}

			if !status.IncludedByToT {
				summaryBuilder.WriteString(fmt.Sprintf("%s: project %s not included in \"ToT\" ConfigGroup\n", builderName, project))
				continue
			}

			if !status.IncludedByBuilder {
				summaryBuilder.WriteString(fmt.Sprintf("%s: project %s not included by builder\n", builderName, project))
				continue
			}

			if status.MatchesMigrationConfig {
				summaryBuilder.WriteString(fmt.Sprintf("%s: project %s migrated\n", builderName, project))
			} else {
				summaryBuilder.WriteString(fmt.Sprintf("%s: project %s not migrated\n", builderName, project))
			}
		}

		if _, err := summaryBuilder.WriteString(
			fmt.Sprintf(
				"%s: %d / %d projects migrated\n",
				builderName, projectsMigrated, totalProjects,
			),
		); err != nil {
			return "", err
		}
	}

	return summaryBuilder.String(), nil
}

// CSV writes a CSV containing each of migrationstatuses to out.
// migrationstatuses are sorted by builder and project.
func CSV(
	ctx context.Context,
	migrationstatuses []*MigrationStatus,
	out io.Writer,
) error {
	writer := csv.NewWriter(out)
	defer writer.Flush()

	if err := writer.Write([]string{
		"builder",
		"project",
		"path",
		"matches migration config",
		"included by ToT",
		"included by builder",
	}); err != nil {
		return err
	}

	sort.SliceStable(migrationstatuses, func(i, j int) bool {
		si := migrationstatuses[i]
		sj := migrationstatuses[j]

		if si.BuilderName != sj.BuilderName {
			return si.BuilderName < sj.BuilderName
		}

		return si.ProjectName < sj.ProjectName
	})

	for _, status := range migrationstatuses {
		if err := writer.Write([]string{
			status.BuilderName,
			status.ProjectName,
			status.ProjectPath,
			strconv.FormatBool(status.MatchesMigrationConfig),
			strconv.FormatBool(status.IncludedByToT),
			strconv.FormatBool(status.IncludedByBuilder),
		}); err != nil {
			return err
		}
	}

	return nil
}
