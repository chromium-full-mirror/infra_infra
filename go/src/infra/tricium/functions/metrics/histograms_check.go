// Copyright 2019 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/data/stringset"
	findingspb "go.chromium.org/luci/common/proto/findings"
)

const (
	category            = "chromium_metrics"
	dateFormat          = "2006-01-02"
	dateMilestoneFormat = "2006-01-02T15:04:05"
	histogramEndTag     = "</histogram>"
	ownerStartTag       = "<owner"
	ownerEndTag         = "</owner"
	variantsEndTag      = "</variants>"

	oneOwnerError                = `It's preferred to list at least two owners, where the second is often a team mailing list or a src/path/to/OWNERS reference: https://chromium.googlesource.com/chromium/src.git/+/HEAD/tools/metrics/histograms/README.md#Owners.`
	firstOwnerTeamError          = `Please list an individual as the primary owner for this metric: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Owners.`
	oneOwnerTeamError            = `Please list an individual as the primary owner for this metric. Note that it's preferred to list at least two owners, where the second is often a team mailing list or a src/path/to/OWNERS reference: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Owners.`
	noExpiryError                = `Please specify an expiry condition for this histogram: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	badExpiryError               = `Could not parse histogram expiry. Please format as YYYY-MM-DD or MXXX: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	pastExpiryWarning            = `This expiry date is in the past. Did you mean to set an expiry date in the future?`
	farExpiryWarning             = `It's a best practice to choose an expiry that is at most one year out: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	dataDiscontinuityWarning     = `This histogram is expired for more than a month. It might have already stopped reporting. If you're extending this histogram, please be careful of data discontinuity: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#extending.`
	neverExpiryInfo              = `The expiry should only be set to "never" in rare cases. Please double-check that this use of "never" is appropriate: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	neverExpiryError             = `The expiry should only be set to "never" in rare cases. If you believe this use of "never" is appropriate, you must include an XML comment describing why, such as <!-- expires-never: "heartbeat" metric (internal: go/uma-heartbeats) -->: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	milestoneFailure             = `Failed to fetch milestone branch date. Please double-check that this milestone is correct, because the tool is currently not able to check for you.`
	unitsHighResolutionWarning   = `Histograms using microseconds should document whether the metric is reported for all clients or only clients with high-resolution clocks. If your histogram logging macro or function calls HistogramBase::AddTimeMicrosecondsGranularity() under the hood, then the metric is reported for only clients with high-resolution clocks. Separately, samples from clients with low-resolution clocks (e.g. on Windows, see TimeTicks::IsHighResolution()) may be as coarse as ~15.6ms.`
	addedNamespaceWarning        = `Are you sure you want to add the namespace %s to histograms.xml? For most new histograms, it's appropriate to re-use one of the existing top-level histogram namespaces. For histogram names, the namespace is defined as everything preceding the first dot '.' in the name.`
	singleElementEnumWarning     = `It looks like this is an enumerated histogram that contains only a single bucket. UMA metrics are difficult to interpret in isolation, so please either add one or more additional buckets that can serve as a baseline for comparison, or document what other metric should be used as a baseline during analysis. https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#enum-histograms.`
	SuffixesDeprecationWarning   = `The <histogram_suffixes> syntax is deprecated. If you're adding a new list of suffixes, please use patterned histograms instead. If you're modifying an existing list of suffixes, please consider migrating that list to use patterned histograms. See https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#patterned-histograms.`
	osxNamespaceDeprecationError = `The namespace "OSX" is deprecated. Prefer adding new Mac histograms to the "Mac" namespace.`
	removedHistogramInfo         = `The following histograms were removed without an obsoletion message: %s. It is preferred to add an obsoletion message when a histogram is removed: https://chromium.googlesource.com/chromium/src/tools/+/HEAD/metrics/histograms/README.md#add-an-obsoletion-message.`
	obsoletionMessageError       = `An obsoletion message has been added to following histograms: %s, but they are not removed. Please double check if there're typos.`
	allRemovedHistogramInfo      = `The following histograms have been removed and obsoleted in this CL: %s.`
	globalObsoletionMessageError = `A CL-level obsoletion message was added but no histogram has been removed in the CL.`
)

var (
	// We need a pattern for matching the histogram start tag because
	// there are other tags that share the "histogram" prefix like "histogram-suffixes"
	histogramStartPattern     = regexp.MustCompile(`^<histogram($|\s|>)`)
	variantsStartPattern      = regexp.MustCompile(`^<variants($|\s|>)`)
	neverExpiryCommentPattern = regexp.MustCompile(`^<!--\s?expires-never`)
	// Match date patterns of format YYYY-MM-DD.
	expiryDatePattern      = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[1-2][0-9]|3[0-1])$`)
	expiryMilestonePattern = regexp.MustCompile(`^M([0-9]{2,3})$`)
	osxNamespaceDeprecated = regexp.MustCompile(`^OSX$`)
	// Match valid summaries for histograms with units=("microseconds", "us", "usec").
	highResolutionUnits        = regexp.MustCompile(`^microsec(onds)?|^us$|^usec.*$`)
	highResolutionUnitsSummary = regexp.MustCompile(`all\suser|(high|low)(\s|-)resolution`)
	expiryAttribute            = regexp.MustCompile(`expires_after="[^"]+"`)
	enumAttribute              = regexp.MustCompile(`enum="[^"]+"`)
	unitsAttribute             = regexp.MustCompile(`units="[^"]+"`)

	// Now is an alias for time.Now, can be overwritten by tests.
	now              = time.Now
	getMilestoneDate = getMilestoneDateImpl

	tags       = []string{ownerEndTag}
	attributes = []*regexp.Regexp{expiryAttribute, enumAttribute, unitsAttribute}
)

// histogram contains all info about a UMA histogram.
type histogram struct {
	Name     string   `xml:"name,attr"`
	Enum     string   `xml:"enum,attr"`
	Units    string   `xml:"units,attr"`
	Expiry   string   `xml:"expires_after,attr"`
	Owners   []string `xml:"owner"`
	Summary  string   `xml:"summary"`
	Tokens   []token  `xml:"token"`
	Obsolete string   `xml:"obsolete"`
}

type variant struct {
	Name     string `xml:"name,attr"`
	Obsolete string `xml:"obsolete"`
}

type token struct {
	Key string `xml:"key,attr"`
	// Variants contains a list of inline <variant>s.
	Variants []variant `xml:"variant"`
	// VariantsName contains the name of the out-of-line <variants>.
	VariantsName string `xml:"variants,attr"`
}

// variants contains info about a <variants>.
type variants struct {
	Name     string    `xml:"name,attr"`
	Variants []variant `xml:"variant"`
}

// metadata contains metadata about histogram tags and required comments.
type metadata struct {
	HistogramLineNum int
	// Handle the line numbers for owner tags separately from other tags and
	// attributes because the <owner> tag can be repeated while the other
	// patterns cannot.
	OwnerStartLineNum int
	// Map from an XML tag to its line number
	tagMap map[string]int
	// Map from an XML attribute to a struct containing its line number,
	// start column number, and end column number
	attributeMap map[*regexp.Regexp]*lineColumnNum

	HasNeverExpiryComment bool
	HistogramBytes        []byte
}

// milestone contains the branch point date of a particular milestone.
type milestone struct {
	Milestone int    `json:"mstone"`
	Date      string `json:"branch_point"`
}

type milestones struct {
	Milestones []milestone `json:"mstones"`
}

// lineColumnNum is used for attributes that are not split across lines
// so there is not a separate start and end line.
type lineColumnNum struct {
	LineNum    int
	StartIndex int
	EndIndex   int
}

type changeMode int

const (
	// ADDED means a line was modified or added to a file.
	ADDED changeMode = iota
	// REMOVED means a line was removed from a file.
	REMOVED
)

func analyzeHistogramFile(f io.Reader, filePath, prevDir string, filesChanged *diffsPerFile, singletonEnums stringset.Set) ([]*findingspb.Finding, stringset.Set, stringset.Set) {
	log.Printf("ANALYZING File: %s", filePath)
	var allFindings []*findingspb.Finding
	// Analyze removed lines in file (if any).
	oldPath := filepath.Join(prevDir, filePath)
	oldFile := openFileOrDie(oldPath)
	defer closeFileOrDie(oldFile)
	var emptySet stringset.Set
	var emptyMap map[string]*histogram
	_, oldHistograms, oldNamespaces, _, oldVariants := analyzeChangedLines(bufio.NewScanner(oldFile), filePath, filesChanged.removedLines[filePath], emptySet, emptyMap, REMOVED)
	// Analyze added lines in file (if any).
	findings, newHistograms, newNamespaces, namespaceLineNums, newVariants := analyzeChangedLines(bufio.NewScanner(f), filePath, filesChanged.addedLines[filePath], singletonEnums, oldHistograms, ADDED)
	allFindings = append(allFindings, findings...)
	// Get the list of added histograms and the list of removed histograms after expansion. Obsolete histograms are excluded.
	addedHistograms, removedHistograms := generateAddedAndRemovedHistograms(newHistograms, oldHistograms, newVariants, oldVariants)
	// Identify if any new namespaces were added.
	allFindings = append(allFindings, generateFindingsForAddedNamespaces(filePath, newNamespaces, oldNamespaces, namespaceLineNums)...)
	return allFindings, addedHistograms, removedHistograms
}

func analyzeHistogramSuffixesFile(f io.Reader, filePath string, filesChanged *diffsPerFile) []*findingspb.Finding {
	log.Printf("ANALYZING File: %s", filePath)
	var findings []*findingspb.Finding
	// Warn on the first changed line whenever users add / update histogram_suffixes_list.
	if linesChanged := filesChanged.addedLines[filePath]; len(linesChanged) > 0 {
		log.Printf("ADDING finding for histogram_suffixes_list at line %d: %s", linesChanged[0], "[WARNING]: Deprecated suffixes")
		findings = append(findings, createHistogramSuffixesFinding(filePath, linesChanged[0]))
	}
	return findings
}

func analyzeCommitMessage(obsoletedHistograms stringset.Set, removedHistograms stringset.Set, globalObsoleteTagAdded bool) []*findingspb.Finding {
	var findings []*findingspb.Finding
	// Check if there's at least one histogram removed when a CL-level obsoletion message is added.
	if len(removedHistograms) == 0 && globalObsoleteTagAdded {
		finding := &findingspb.Finding{
			Category:      category,
			Message:       globalObsoletionMessageError,
			SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
			Location: &findingspb.Location{
				FilePath: "/COMMIT_MSG",
			},
		}
		findings = append(findings, finding)
	}

	// Check if there's any obsoletion message added without a corresponding histogram removed.
	obsoletedWithoutRemovalHistograms := obsoletedHistograms.Difference(removedHistograms).ToSlice()
	if len(obsoletedWithoutRemovalHistograms) > 0 {
		finding := &findingspb.Finding{
			Category:      category,
			Message:       fmt.Sprintf(obsoletionMessageError, strings.Join(obsoletedWithoutRemovalHistograms, ", ")),
			SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
			Location: &findingspb.Location{
				FilePath: "/COMMIT_MSG",
			},
		}
		findings = append(findings, finding)
	}

	// If a CL-level obsoletion message was added, we don't need to check if there's any histogram removed without
	// an obsoletion message.
	if globalObsoleteTagAdded {
		return findings
	}

	// Check if there's any histogram removed without an obsoletion message.
	removedWithoutMessageHistograms := removedHistograms.Difference(obsoletedHistograms).ToSlice()
	if len(removedWithoutMessageHistograms) > 0 {
		finding := &findingspb.Finding{
			Category:      category,
			Message:       fmt.Sprintf(removedHistogramInfo, strings.Join(removedWithoutMessageHistograms, ", ")),
			SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_INFO,
			Location: &findingspb.Location{
				FilePath: "/COMMIT_MSG",
			},
		}
		findings = append(findings, finding)
	}
	return findings
}

// analyzeChangedLines analyzes a version of the file and returns:
// 1. A list of findings generated from analyzing changed histograms.
// 2. A map containing all histograms keyed by their names in the file.
// 3. A set containing all the names of namespaces in the file.
// 4. A map from namespace to line number.
// 5. A map containing all out-of-line variants keyed by their names in the file.
func analyzeChangedLines(scanner *bufio.Scanner, path string, linesChanged []int, singletonEnums stringset.Set, oldHistograms map[string]*histogram, mode changeMode) ([]*findingspb.Finding, map[string]*histogram, stringset.Set, map[string]int, map[string]*variants) {
	var findings []*findingspb.Finding
	// meta is a struct that holds line numbers of different tags in histogram.
	var meta *metadata
	// currHistogram is a buffer that holds the current histogram.
	var currHistogram []byte
	// histogramStart is the starting line number for the current histogram.
	var histogramStart int
	// currVariants is a buffer that holds the current variants.
	var currVariants []byte
	// variantsStart is the starting line number for the current variants.
	var variantsStart int
	// If any line in the histogram showed up as an added or removed line in the diff.
	var histogramChanged bool
	allHistograms := make(map[string]*histogram)
	allVariants := make(map[string]*variants)
	namespaces := make(stringset.Set)
	namespaceLineNums := make(map[string]int)
	lineNum := 1
	changedIndex := 0
	for scanner.Scan() {
		// Copying scanner.Scan() is necessary to ensure the scanner does not
		// overwrite the memory that stores currHistogram.
		newBytes := make([]byte, len(scanner.Bytes()))
		copy(newBytes, scanner.Bytes())
		// We don't need to check top level comments, etc.
		if currHistogram != nil {
			// Add line to currHistogram if currently between some histogram tags.
			currHistogram = append(currHistogram, newBytes...)
		} else if currVariants != nil {
			// Add line to currVariants if currently between some variants tags.
			currVariants = append(currVariants, newBytes...)
		}
		line := strings.TrimSpace(scanner.Text())
		if histogramStartPattern.MatchString(line) {
			// Initialize currHistogram and metadata when a new histogram is encountered.
			histogramStart = lineNum
			currHistogram = newBytes
			meta = newMetadata(lineNum)
			histogramChanged = false
		} else if variantsStartPattern.MatchString(line) {
			// Initialize currVariants and record the starting line number when a new variants is encountered.
			currVariants = newBytes
			variantsStart = lineNum
		}
		if changedIndex < len(linesChanged) && lineNum == linesChanged[changedIndex] {
			histogramChanged = true
			changedIndex++
		}
		if strings.HasPrefix(line, variantsEndTag) {
			// Analyze entire variants after variants end tag is encountered.
			variants := bytesToVariants(currVariants, variantsStart)
			allVariants[variants.Name] = variants
			currVariants = nil
		}
		if currHistogram == nil {
			lineNum++
			continue
		}
		if strings.HasPrefix(line, histogramEndTag) {
			// Analyze entire histogram after histogram end tag is encountered.
			hist := bytesToHistogram(currHistogram, meta)
			namespace := parseNamespaceFromHistogramName(hist.Name)
			namespaces.Add(namespace)
			allHistograms[hist.Name] = hist
			if namespaceLineNums[namespace] == 0 {
				namespaceLineNums[namespace] = histogramStart
			}
			if histogramChanged {
				// Only check new (added) histograms are correct.
				if mode == ADDED {
					findings = append(findings, checkHistogram(path, hist, meta, singletonEnums, oldHistograms)...)
				}
			}
			currHistogram = nil
		} else if strings.HasPrefix(line, ownerStartTag) && meta.OwnerStartLineNum == histogramStart {
			meta.OwnerStartLineNum = lineNum
		} else if neverExpiryCommentPattern.MatchString(line) {
			meta.HasNeverExpiryComment = true
		}
		for _, tag := range tags {
			if strings.Contains(line, tag) {
				meta.tagMap[tag] = lineNum
			}
		}
		for _, attribute := range attributes {
			indices := attribute.FindIndex([]byte(scanner.Text()))
			if indices != nil {
				meta.attributeMap[attribute] = &lineColumnNum{lineNum, indices[0], indices[1]}
			}
		}
		lineNum++
	}
	return findings, allHistograms, namespaces, namespaceLineNums, allVariants
}

func parseNamespaceFromHistogramName(histogramName string) string {
	return strings.SplitN(histogramName, ".", 2)[0]
}

func checkHistogram(path string, hist *histogram, meta *metadata, singletonEnums stringset.Set, oldHistograms map[string]*histogram) []*findingspb.Finding {
	var findings []*findingspb.Finding
	findings = append(findings, checkExpiry(path, hist, meta, oldHistograms)...)
	if finding := checkOwners(path, hist, meta); finding != nil {
		findings = append(findings, finding)
	}
	if finding := checkUnits(path, hist, meta); finding != nil {
		findings = append(findings, finding)
	}
	if finding := checkEnums(path, hist, meta, singletonEnums); finding != nil {
		findings = append(findings, finding)
	}
	if finding := checkDeprecatedNamespaces(path, hist, meta); finding != nil {
		findings = append(findings, finding)
	}
	return findings
}

func checkDeprecatedNamespaces(path string, hist *histogram, meta *metadata) *findingspb.Finding {
	namespace := parseNamespaceFromHistogramName(hist.Name)
	if osxNamespaceDeprecated.MatchString(namespace) {
		finding := &findingspb.Finding{
			Category:      category,
			Message:       osxNamespaceDeprecationError,
			SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_ERROR,
			Location: &findingspb.Location{
				FilePath: path,
				Range: &findingspb.Location_Range{
					StartLine: int32(meta.HistogramLineNum),
					EndLine:   int32(meta.HistogramLineNum) + 1,
				},
			},
		}
		log.Printf("ADDING finding for %s at line %d: %s", hist.Name, finding.Location.Range.StartLine, "[ERROR]: Deprecated Namespace")
		return finding
	}
	return nil
}

func bytesToHistogram(histBytes []byte, meta *metadata) *histogram {
	var hist *histogram
	if err := xml.Unmarshal(histBytes, &hist); err != nil {
		log.Panicf("WARNING: Failed to unmarshal histogram at line %d", meta.HistogramLineNum)
	}
	return hist
}

func bytesToVariants(variantsBytes []byte, variantsStart int) *variants {
	var variants *variants
	if err := xml.Unmarshal(variantsBytes, &variants); err != nil {
		log.Panicf("WARNING: Failed to unmarshal variants at line %s", string(variantsBytes))
	}
	return variants
}

func checkOwners(path string, hist *histogram, meta *metadata) *findingspb.Finding {
	var finding *findingspb.Finding

	// Check that there is more than 1 owner
	if len(hist.Owners) <= 1 {
		finding = createOwnerFinding(oneOwnerError, path, meta)
		log.Printf("ADDING finding for %s at line %d: %s", hist.Name, finding.Location.Range.StartLine, "[ERROR]: One Owner")
	}
	// Check first owner is a not a team or OWNERS file.
	if len(hist.Owners) > 0 && (strings.Contains(hist.Owners[0], "-") || strings.Contains(hist.Owners[0], "OWNERS")) {
		if finding != nil {
			finding.Message = oneOwnerTeamError
		} else {
			finding = createOwnerFinding(firstOwnerTeamError, path, meta)
		}
		log.Printf("ADDING finding for %s at line %d: %s", hist.Name, finding.Location.Range.StartLine, "[ERROR]: First Owner Team")
	}
	return finding
}

func createOwnerFinding(message, path string, meta *metadata) *findingspb.Finding {
	return &findingspb.Finding{
		Category:      category,
		Message:       message,
		SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
		Location: &findingspb.Location{
			FilePath: path,
			Range: &findingspb.Location_Range{
				StartLine: int32(meta.OwnerStartLineNum),
				EndLine:   int32(meta.tagMap[ownerEndTag]),
			},
		},
	}
}

func createHistogramSuffixesFinding(path string, lineNum int) *findingspb.Finding {
	return &findingspb.Finding{
		Category:      category,
		Message:       SuffixesDeprecationWarning,
		SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
		Location: &findingspb.Location{
			FilePath: path,
			Range: &findingspb.Location_Range{
				StartLine: int32(lineNum),
				EndLine:   int32(lineNum),
			},
		},
	}
}

func checkUnits(path string, hist *histogram, meta *metadata) *findingspb.Finding {
	if highResolutionUnits.MatchString(hist.Units) && !highResolutionUnitsSummary.MatchString(hist.Summary) {
		unitsLine := meta.attributeMap[unitsAttribute]
		finding := &findingspb.Finding{
			Category:      category,
			Message:       unitsHighResolutionWarning,
			SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
			Location: &findingspb.Location{
				FilePath: path,
				Range: &findingspb.Location_Range{
					StartLine:   int32(unitsLine.LineNum),
					EndLine:     int32(unitsLine.LineNum),
					StartColumn: int32(unitsLine.StartIndex),
					EndColumn:   int32(unitsLine.EndIndex),
				},
			},
		}
		log.Printf("ADDING finding for %s at line %d: %s", hist.Name, finding.Location.Range.StartLine, "[ERROR]: Units Microseconds Bad Summary")
		return finding
	}
	return nil
}

func checkExpiry(path string, hist *histogram, meta *metadata, oldHistograms map[string]*histogram) []*findingspb.Finding {
	var findingMessage string
	var logMessage string
	var sevLevel findingspb.Finding_SeverityLevel
	var expiryFindings []*findingspb.Finding
	expiry := hist.Expiry
	// Check if there is any data discontinuity when |hist| already exists and is already
	// expired for more than a month.
	if oldHist, ok := oldHistograms[hist.Name]; ok {
		// Show a warning if the histogram has been expired for more than 30 days.
		if hasExpiredBy(oldHist, 30) {
			expiryFindings = append(expiryFindings, createExpiryFinding(dataDiscontinuityWarning, path, findingspb.Finding_SEVERITY_LEVEL_WARNING, meta))
		}
	}
	if expiry == "" {
		findingMessage = noExpiryError
		logMessage = "[ERROR]: No Expiry"
		sevLevel = findingspb.Finding_SEVERITY_LEVEL_ERROR
	} else if expiry == "never" {
		if !meta.HasNeverExpiryComment {
			findingMessage = neverExpiryError
			logMessage = "[ERROR]: Never Expiry, No Comment"
			sevLevel = findingspb.Finding_SEVERITY_LEVEL_ERROR
		} else {
			findingMessage = neverExpiryInfo
			logMessage = "[INFO]: Never Expiry"
			sevLevel = findingspb.Finding_SEVERITY_LEVEL_INFO
		}
	} else if expiry != "" {
		if inputDate, msg, log, level, ok := getExpiryDate(expiry); ok {
			findingMessage, logMessage, sevLevel = processExpiryDateDiff(inputDate)
		} else {
			findingMessage = msg
			logMessage = log
			sevLevel = level
		}
	}
	if findingMessage != "" {
		expiryFindings = append(expiryFindings, createExpiryFinding(findingMessage, path, sevLevel, meta))
		log.Printf("ADDING finding for %s at line %d: %s", hist.Name, meta.HistogramLineNum, logMessage)
	}
	return expiryFindings
}

func getExpiryDate(expiry string) (inputDate time.Time, findingMessage string, logMessage string, level findingspb.Finding_SeverityLevel, ok bool) {
	var err error
	ok = true
	dateMatch := expiryDatePattern.MatchString(expiry)
	milestoneMatch := expiryMilestonePattern.MatchString(expiry)
	if dateMatch {
		if inputDate, err = time.Parse(dateFormat, expiry); err != nil {
			ok = false
			log.Panicf("Failed to parse expiry date: %v", err)
		}
	} else if milestoneMatch {
		milestone, err := strconv.Atoi(expiry[1:])
		if err != nil {
			ok = false
			log.Panicf("Failed to convert input milestone to integer: %v", err)
		}
		if inputDate, err = getMilestoneDate(milestone); err != nil {
			ok = false
			findingMessage = milestoneFailure
			logMessage = fmt.Sprintf("[WARNING] Milestone Fetch Failure: %v", err)
			level = findingspb.Finding_SEVERITY_LEVEL_WARNING
		}
	} else {
		ok = false
		findingMessage = badExpiryError
		logMessage = "[ERROR]: Expiry condition badly formatted"
		level = findingspb.Finding_SEVERITY_LEVEL_ERROR
	}
	return
}

func processExpiryDateDiff(inputDate time.Time) (findingMessage string, logMessage string, level findingspb.Finding_SeverityLevel) {
	dateDiff := int(inputDate.Sub(now()).Hours() / 24)
	if dateDiff < 0 {
		return pastExpiryWarning, "[WARNING]: Expiry in past", findingspb.Finding_SEVERITY_LEVEL_WARNING
	} else if dateDiff >= 420 {
		// Use a threshold of 420 days to give users a 2-month grace period for
		// expiry dates past 1 year. When a histogram is nearing expiry, an
		// automated system will file a bug reminding developers to update the
		// expiry date if the histogram is still relevant. This automation runs
		// about a month or two before the histogram will expire, and it's common
		// for developers to simply bump the expiry year, without changing the month
		// nor day.
		return farExpiryWarning, "[WARNING]: Expiry past one year", findingspb.Finding_SEVERITY_LEVEL_WARNING
	}
	return "", "", findingspb.Finding_SEVERITY_LEVEL_UNSPECIFIED
}

func getMilestoneDateImpl(milestone int) (time.Time, error) {
	var milestoneDate time.Time
	url := fmt.Sprintf("https://chromiumdash.appspot.com/fetch_milestone_schedule?mstone=%d", milestone)
	newMilestones, err := milestoneRequest(url)
	if err != nil {
		return milestoneDate, err
	}
	dateString := newMilestones.Milestones[0].Date
	log.Printf("Fetched branch date %s for milestone %d", dateString, milestone)
	milestoneDate, err = time.Parse(dateMilestoneFormat, dateString)
	if err != nil {
		log.Panicf("Failed to parse milestone date: %v", err)
	}
	return milestoneDate, nil
}

func milestoneRequest(url string) (milestones, error) {
	newMilestones := milestones{}
	milestoneClient := http.Client{
		Timeout: time.Second * 2,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return newMilestones, err
	}
	res, err := milestoneClient.Do(req)
	if err != nil {
		return newMilestones, err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return newMilestones, err
	}
	err = json.Unmarshal(body, &newMilestones)
	if err != nil {
		return newMilestones, err
	}
	if len(newMilestones.Milestones) == 0 {
		err = fmt.Errorf("No milestone data returned for query; response: %s", body)
		return newMilestones, err
	}
	return newMilestones, nil
}

func createExpiryFinding(message, path string, level findingspb.Finding_SeverityLevel, meta *metadata) *findingspb.Finding {
	expiryLine := meta.attributeMap[expiryAttribute]
	log.Printf("ADDING finding at line %d: %s", expiryLine.LineNum, message)
	return &findingspb.Finding{
		Category:      category,
		Message:       message,
		SeverityLevel: level,
		Location: &findingspb.Location{
			FilePath: path,
			Range: &findingspb.Location_Range{
				StartLine:   int32(expiryLine.LineNum),
				EndLine:     int32(expiryLine.LineNum),
				StartColumn: int32(expiryLine.StartIndex),
				EndColumn:   int32(expiryLine.EndIndex),
			},
		},
	}
}

func checkEnums(path string, hist *histogram, meta *metadata, singletonEnums stringset.Set) *findingspb.Finding {
	if singletonEnums.Has(hist.Enum) && !strings.Contains(hist.Summary, "baseline") {
		enumLine := meta.attributeMap[enumAttribute]
		log.Printf("ADDING finding for %s at line %d: %s", hist.Name, enumLine.LineNum, "Single Element Enum No Baseline")
		return &findingspb.Finding{
			Category:      category,
			Message:       singleElementEnumWarning,
			SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
			Location: &findingspb.Location{
				FilePath: path,
				Range: &findingspb.Location_Range{
					StartLine:   int32(enumLine.LineNum),
					EndLine:     int32(enumLine.LineNum),
					StartColumn: int32(enumLine.StartIndex),
					EndColumn:   int32(enumLine.EndIndex),
				},
			},
		}
	}
	return nil
}

func generateAddedAndRemovedHistograms(newHistograms map[string]*histogram, oldHistograms map[string]*histogram, newVariants map[string]*variants, oldVariants map[string]*variants) (stringset.Set, stringset.Set) {
	newHistogramNames := expandHistograms(newHistograms, newVariants)
	oldHistogramNames := expandHistograms(oldHistograms, oldVariants)
	allAddedHistograms := newHistogramNames.Difference(oldHistogramNames)
	allRemovedHistograms := oldHistogramNames.Difference(newHistogramNames)
	return allAddedHistograms, allRemovedHistograms
}

func generateFindingsForAddedNamespaces(path string, newNamespaces stringset.Set, oldNamespaces stringset.Set, namespaceLineNums map[string]int) []*findingspb.Finding {
	var findings []*findingspb.Finding
	allAddedNamespaces := newNamespaces.Difference(oldNamespaces).ToSlice()
	sort.Strings(allAddedNamespaces)
	for _, namespace := range allAddedNamespaces {
		finding := &findingspb.Finding{
			Category:      category,
			Message:       fmt.Sprintf(addedNamespaceWarning, namespace),
			SeverityLevel: findingspb.Finding_SEVERITY_LEVEL_WARNING,
			Location: &findingspb.Location{
				FilePath: path,
				Range: &findingspb.Location_Range{
					StartLine: int32(namespaceLineNums[namespace]),
					EndLine:   int32(namespaceLineNums[namespace]),
				},
			},
		}
		log.Printf("ADDING finding for %s at line %d: %s", namespace, finding.Location.Range.StartLine, "[WARNING]: Added Namespace")
		findings = append(findings, finding)
	}
	return findings
}

// newMetadata is a constructor for creating a Metadata struct with defaultLineNum.
func newMetadata(defaultLineNum int) *metadata {
	tagMap := make(map[string]int)
	attributeMap := make(map[*regexp.Regexp]*lineColumnNum)
	for _, tag := range tags {
		tagMap[tag] = defaultLineNum
	}
	for _, attribute := range attributes {
		attributeMap[attribute] = &lineColumnNum{defaultLineNum, 0, 0}
	}
	return &metadata{
		HistogramLineNum:  defaultLineNum,
		OwnerStartLineNum: defaultLineNum,
		tagMap:            tagMap,
		attributeMap:      attributeMap,
	}
}

// expandHistograms generates a map containing all histograms that are not obsolete keyed by their
// names after expansion.
func expandHistograms(hists map[string]*histogram, variants map[string]*variants) stringset.Set {
	histogramNames := make(stringset.Set)
	for name := range hists {
		hist := hists[name]
		// Skip the histogram if it's already obsolete.
		if hist.Obsolete != "" {
			continue
		}
		if hist.Tokens == nil {
			histogramNames.Add(name)
		} else {
			currHistogramNames := make(stringset.Set)
			currHistogramNames.Add(name)
			for _, token := range hist.Tokens {
				newHistogramNames := make(stringset.Set)
				for patternedName := range currHistogramNames {
					currVariants := token.Variants
					// If there's no inline variants defined, we'll get the variants from
					// out-of-line <variants>.
					if currVariants == nil {
						currVariants = variants[token.VariantsName].Variants
					}
					for _, variant := range currVariants {
						// Skip obsolete variants.
						if variant.Obsolete != "" {
							continue
						}
						newHistogramNames.Add(strings.ReplaceAll(patternedName, "{"+token.Key+"}", variant.Name))
					}
				}
				currHistogramNames = newHistogramNames
			}
			histogramNames = histogramNames.Union(currHistogramNames)
		}
	}
	return histogramNames
}

// hasExpiredBy returns whether the given histogram has been expired for more than the give number of days.
func hasExpiredBy(hist *histogram, numDays int) bool {
	if hist.Expiry != "" {
		if inputDate, _, _, _, ok := getExpiryDate(hist.Expiry); ok {
			dateDiff := int(inputDate.Sub(now()).Hours() / 24)
			return dateDiff < -1*numDays
		}
	}
	return false
}
