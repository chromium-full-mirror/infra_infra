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
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.chromium.org/luci/common/data/stringset"

	tricium "infra/tricium/api/v1"
)

const (
	category            = "Metrics"
	dateFormat          = "2006-01-02"
	dateMilestoneFormat = "2006-01-02T15:04:05"
	histogramEndTag     = "</histogram>"
	ownerStartTag       = "<owner"
	ownerEndTag         = "</owner"
	variantsEndTag      = "</variants>"

	oneOwnerError                = `[WARNING] It's preferred to list at least two owners, where the second is often a team mailing list or a src/path/to/OWNERS reference: https://chromium.googlesource.com/chromium/src.git/+/HEAD/tools/metrics/histograms/README.md#Owners.`
	firstOwnerTeamError          = `[WARNING] Please list an individual as the primary owner for this metric: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Owners.`
	oneOwnerTeamError            = `[WARNING] Please list an individual as the primary owner for this metric. Note that it's preferred to list at least two owners, where the second is often a team mailing list or a src/path/to/OWNERS reference: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Owners.`
	noExpiryError                = `[ERROR] Please specify an expiry condition for this histogram: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	badExpiryError               = `[ERROR] Could not parse histogram expiry. Please format as YYYY-MM-DD or MXXX: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	pastExpiryWarning            = `[WARNING] This expiry date is in the past. Did you mean to set an expiry date in the future?`
	farExpiryWarning             = `[WARNING] It's a best practice to choose an expiry that is at most one year out: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	dataDiscontinuityWarning     = `[WARNING] This histogram is expired for more than a month. It might have already stopped reporting. If you're extending this histogram, please be careful of data discontinuity: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#extending.`
	neverExpiryInfo              = `[INFO] The expiry should only be set to "never" in rare cases. Please double-check that this use of "never" is appropriate: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	neverExpiryError             = `[ERROR] The expiry should only be set to "never" in rare cases. If you believe this use of "never" is appropriate, you must include an XML comment describing why, such as <!-- expires-never: "heartbeat" metric (internal: go/uma-heartbeats) -->: https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#Histogram-Expiry.`
	milestoneFailure             = `[WARNING] Tricium failed to fetch milestone branch date. Please double-check that this milestone is correct, because the tool is currently not able to check for you.`
	unitsHighResolutionWarning   = `[WARNING] Histograms using microseconds should document whether the metric is reported for all clients or only clients with high-resolution clocks. If your histogram logging macro or function calls HistogramBase::AddTimeMicrosecondsGranularity() under the hood, then the metric is reported for only clients with high-resolution clocks. Separately, samples from clients with low-resolution clocks (e.g. on Windows, see TimeTicks::IsHighResolution()) may be as coarse as ~15.6ms.`
	addedNamespaceWarning        = `[WARNING] Are you sure you want to add the namespace %s to histograms.xml? For most new histograms, it's appropriate to re-use one of the existing top-level histogram namespaces. For histogram names, the namespace is defined as everything preceding the first dot '.' in the name.`
	singleElementEnumWarning     = `[WARNING] It looks like this is an enumerated histogram that contains only a single bucket. UMA metrics are difficult to interpret in isolation, so please either add one or more additional buckets that can serve as a baseline for comparison, or document what other metric should be used as a baseline during analysis. https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#enum-histograms.`
	SuffixesDeprecationWarning   = `[WARNING] The <histogram_suffixes> syntax is deprecated. If you're adding a new list of suffixes, please use patterned histograms instead. If you're modifying an existing list of suffixes, please consider migrating that list to use patterned histograms. See https://chromium.googlesource.com/chromium/src/+/HEAD/tools/metrics/histograms/README.md#patterned-histograms.`
	osxNamespaceDeprecationError = `[ERROR] The namespace "OSX" is deprecated. Prefer adding new Mac histograms to the "Mac" namespace.`
	removedHistogramInfo         = `[INFO] The following histograms were removed without an obsoletion message: %s. It is preferred to add an obsoletion message when a histogram is removed: https://chromium.googlesource.com/chromium/src/tools/+/HEAD/metrics/histograms/README.md#add-an-obsoletion-message.`
	obsoletionMessageError       = `[WARNING] An obsoletion message has been added to following histograms: %s, but they are not removed. Please double check if there're typos.`
	allRemovedHistogramInfo      = `[INFO] The following histograms have been removed and obsoleted in this CL: %s.`
	globalObsoletionMessageError = `[WARNING] A CL-level obsoletion message was added but no histogram has been removed in the CL.`
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

func analyzeHistogramFile(f io.Reader, filePath, prevDir string, filesChanged *diffsPerFile, singletonEnums stringset.Set) ([]*tricium.Data_Comment, stringset.Set, stringset.Set) {
	log.Printf("ANALYZING File: %s", filePath)
	var allComments []*tricium.Data_Comment
	// Analyze removed lines in file (if any).
	oldPath := filepath.Join(prevDir, filePath)
	oldFile := openFileOrDie(oldPath)
	defer closeFileOrDie(oldFile)
	var emptySet stringset.Set
	var emptyMap map[string]*histogram
	_, oldHistograms, oldNamespaces, _, oldVariants := analyzeChangedLines(bufio.NewScanner(oldFile), filePath, filesChanged.removedLines[filePath], emptySet, emptyMap, REMOVED)
	// Analyze added lines in file (if any).
	comments, newHistograms, newNamespaces, namespaceLineNums, newVariants := analyzeChangedLines(bufio.NewScanner(f), filePath, filesChanged.addedLines[filePath], singletonEnums, oldHistograms, ADDED)
	allComments = append(allComments, comments...)
	// Get the list of added histograms and the list of removed histograms after expansion. Obsolete histograms are excluded.
	addedHistograms, removedHistograms := generateAddedAndRemovedHistograms(newHistograms, oldHistograms, newVariants, oldVariants)
	// Identify if any new namespaces were added.
	allComments = append(allComments, generateCommentsForAddedNamespaces(filePath, newNamespaces, oldNamespaces, namespaceLineNums)...)
	return showAllComments(allComments), addedHistograms, removedHistograms
}

func analyzeHistogramSuffixesFile(f io.Reader, filePath string, filesChanged *diffsPerFile) []*tricium.Data_Comment {
	log.Printf("ANALYZING File: %s", filePath)
	var comments []*tricium.Data_Comment
	// Warn on the first changed line whenever users add / update histogram_suffixes_list.
	if linesChanged := filesChanged.addedLines[filePath]; len(linesChanged) > 0 {
		log.Printf("ADDING Comment for histogram_suffixes_list at line %d: %s", linesChanged[0], "[WARNING]: Deprecated suffixes")
		comments = append(comments, createHistogramSuffixesComment(filePath, linesChanged[0]))
	}
	return showAllComments(comments)
}

func analyzeCommitMessage(obsoletedHistograms stringset.Set, removedHistograms stringset.Set, globalObsoleteTagAdded bool) []*tricium.Data_Comment {
	var comments []*tricium.Data_Comment
	// Check if there's at least one histogram removed when a CL-level obsoletion message is added.
	if len(removedHistograms) == 0 && globalObsoleteTagAdded {
		comment := &tricium.Data_Comment{
			Category: category + "/Obsolete",
			Message:  globalObsoletionMessageError,
		}
		comments = append(comments, comment)
	}

	// Check if there's any obsoletion message added without a corresponding histogram removed.
	obsoletedWithoutRemovalHistograms := obsoletedHistograms.Difference(removedHistograms).ToSlice()
	if len(obsoletedWithoutRemovalHistograms) > 0 {
		comment := &tricium.Data_Comment{
			Category: category + "/Obsolete",
			Message:  fmt.Sprintf(obsoletionMessageError, strings.Join(obsoletedWithoutRemovalHistograms, ", ")),
		}
		comments = append(comments, comment)
	}

	// If a CL-level obsoletion message was added, we don't need to check if there's any histogram removed without
	// an obsoletion message.
	if globalObsoleteTagAdded {
		return comments
	}

	// Check if there's any histogram removed without an obsoletion message.
	removedWithoutMessageHistograms := removedHistograms.Difference(obsoletedHistograms).ToSlice()
	if len(removedWithoutMessageHistograms) > 0 {
		comment := &tricium.Data_Comment{
			Category: category + "/Obsolete",
			Message:  fmt.Sprintf(removedHistogramInfo, strings.Join(removedWithoutMessageHistograms, ", ")),
		}
		comments = append(comments, comment)
	}
	return comments
}

// analyzeChangedLines analyzes a version of the file and returns:
// 1. A list of comments generated from analyzing changed histograms.
// 2. A map containing all histograms keyed by their names in the file.
// 3. A set containing all the names of namespaces in the file.
// 4. A map from namespace to line number.
// 5. A map containing all out-of-line variants keyed by their names in the file.
func analyzeChangedLines(scanner *bufio.Scanner, path string, linesChanged []int, singletonEnums stringset.Set, oldHistograms map[string]*histogram, mode changeMode) ([]*tricium.Data_Comment, map[string]*histogram, stringset.Set, map[string]int, map[string]*variants) {
	var comments []*tricium.Data_Comment
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
					comments = append(comments, checkHistogram(path, hist, meta, singletonEnums, oldHistograms)...)
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
	return comments, allHistograms, namespaces, namespaceLineNums, allVariants
}

func parseNamespaceFromHistogramName(histogramName string) string {
	return strings.SplitN(histogramName, ".", 2)[0]
}

func checkHistogram(path string, hist *histogram, meta *metadata, singletonEnums stringset.Set, oldHistograms map[string]*histogram) []*tricium.Data_Comment {
	var comments []*tricium.Data_Comment
	comments = append(comments, checkExpiry(path, hist, meta, oldHistograms)...)
	if comment := checkOwners(path, hist, meta); comment != nil {
		comments = append(comments, comment)
	}
	if comment := checkUnits(path, hist, meta); comment != nil {
		comments = append(comments, comment)
	}
	if comment := checkEnums(path, hist, meta, singletonEnums); comment != nil {
		comments = append(comments, comment)
	}
	if comment := checkDeprecatedNamespaces(path, hist, meta); comment != nil {
		comments = append(comments, comment)
	}
	return comments
}

func checkDeprecatedNamespaces(path string, hist *histogram, meta *metadata) *tricium.Data_Comment {
	namespace := parseNamespaceFromHistogramName(hist.Name)
	if osxNamespaceDeprecated.MatchString(namespace) {
		comment := &tricium.Data_Comment{
			Category:  category + "/Namespace",
			Message:   osxNamespaceDeprecationError,
			Path:      path,
			StartLine: int32(meta.HistogramLineNum),
			EndLine:   int32(meta.HistogramLineNum) + 1,
		}
		log.Printf("ADDING Comment for %s at line %d: %s", hist.Name, comment.StartLine, "[ERROR]: Deprecated Namespace")
		return comment
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

func checkOwners(path string, hist *histogram, meta *metadata) *tricium.Data_Comment {
	var comment *tricium.Data_Comment

	// Check that there is more than 1 owner
	if len(hist.Owners) <= 1 {
		comment = createOwnerComment(oneOwnerError, path, meta)
		log.Printf("ADDING Comment for %s at line %d: %s", hist.Name, comment.StartLine, "[ERROR]: One Owner")
	}
	// Check first owner is a not a team or OWNERS file.
	if len(hist.Owners) > 0 && (strings.Contains(hist.Owners[0], "-") || strings.Contains(hist.Owners[0], "OWNERS")) {
		if comment != nil {
			comment.Message = oneOwnerTeamError
		} else {
			comment = createOwnerComment(firstOwnerTeamError, path, meta)
		}
		log.Printf("ADDING Comment for %s at line %d: %s", hist.Name, comment.StartLine, "[ERROR]: First Owner Team")
	}
	return comment
}

func createOwnerComment(message, path string, meta *metadata) *tricium.Data_Comment {
	return &tricium.Data_Comment{
		Category:  category + "/Owners",
		Message:   message,
		Path:      path,
		StartLine: int32(meta.OwnerStartLineNum),
		EndLine:   int32(meta.tagMap[ownerEndTag]),
	}
}

func createHistogramSuffixesComment(path string, lineNum int) *tricium.Data_Comment {
	return &tricium.Data_Comment{
		Category:  category + "/Suffixes",
		Message:   SuffixesDeprecationWarning,
		Path:      path,
		StartLine: int32(lineNum),
		EndLine:   int32(lineNum),
	}
}

func checkUnits(path string, hist *histogram, meta *metadata) *tricium.Data_Comment {
	if highResolutionUnits.MatchString(hist.Units) && !highResolutionUnitsSummary.MatchString(hist.Summary) {
		unitsLine := meta.attributeMap[unitsAttribute]
		comment := &tricium.Data_Comment{
			Category:  category + "/Units",
			Message:   unitsHighResolutionWarning,
			Path:      path,
			StartLine: int32(unitsLine.LineNum),
			EndLine:   int32(unitsLine.LineNum),
			StartChar: int32(unitsLine.StartIndex),
			EndChar:   int32(unitsLine.EndIndex),
		}
		log.Printf("ADDING Comment for %s at line %d: %s", hist.Name, comment.StartLine, "[ERROR]: Units Microseconds Bad Summary")
		return comment
	}
	return nil
}

func checkExpiry(path string, hist *histogram, meta *metadata, oldHistograms map[string]*histogram) []*tricium.Data_Comment {
	var commentMessage string
	var logMessage string
	var expiryComments []*tricium.Data_Comment
	expiry := hist.Expiry
	// Check if there is any data discontinuity when |hist| already exists and is already
	// expired for more than a month.
	if oldHist, ok := oldHistograms[hist.Name]; ok {
		// Show a Tricium warning if the histogram has been expired for more than 30 days.
		if hasExpiredBy(oldHist, 30) {
			expiryComments = append(expiryComments, createExpiryComment(dataDiscontinuityWarning, path, meta))
		}
	}
	if expiry == "" {
		commentMessage = noExpiryError
		logMessage = "[ERROR]: No Expiry"
	} else if expiry == "never" {
		if !meta.HasNeverExpiryComment {
			commentMessage = neverExpiryError
			logMessage = "[ERROR]: Never Expiry, No Comment"
		} else {
			commentMessage = neverExpiryInfo
			logMessage = "[INFO]: Never Expiry"
		}
	} else if expiry != "" {
		if inputDate, comment, log, ok := getExpiryDate(expiry); ok {
			commentMessage, logMessage = processExpiryDateDiff(inputDate)
		} else {
			commentMessage = comment
			logMessage = log
		}
	}
	if commentMessage != "" {
		expiryComments = append(expiryComments, createExpiryComment(commentMessage, path, meta))
		log.Printf("ADDING Comment for %s at line %d: %s", hist.Name, meta.HistogramLineNum, logMessage)
	}
	return expiryComments
}

func getExpiryDate(expiry string) (inputDate time.Time, commentMessage string, logMessage string, ok bool) {
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
			commentMessage = milestoneFailure
			logMessage = fmt.Sprintf("[WARNING] Milestone Fetch Failure: %v", err)
		}
	} else {
		ok = false
		commentMessage = badExpiryError
		logMessage = "[ERROR]: Expiry condition badly formatted"
	}
	return
}

func processExpiryDateDiff(inputDate time.Time) (commentMessage string, logMessage string) {
	dateDiff := int(inputDate.Sub(now()).Hours() / 24)
	if dateDiff < 0 {
		commentMessage = pastExpiryWarning
		logMessage = "[WARNING]: Expiry in past"
	} else if dateDiff >= 420 {
		// Use a threshold of 420 days to give users a 2-month grace period for
		// expiry dates past 1 year. When a histogram is nearing expiry, an
		// automated system will file a bug reminding developers to update the
		// expiry date if the histogram is still relevant. This automation runs
		// about a month or two before the histogram will expire, and it's common
		// for developers to simply bump the expiry year, without changing the month
		// nor day.
		commentMessage = farExpiryWarning
		logMessage = "[WARNING]: Expiry past one year"
	}
	return
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
	body, err := ioutil.ReadAll(res.Body)
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

func createExpiryComment(message, path string, meta *metadata) *tricium.Data_Comment {
	expiryLine := meta.attributeMap[expiryAttribute]
	log.Printf("ADDING Comment at line %d: %s", expiryLine.LineNum, message)
	return &tricium.Data_Comment{
		Category:  category + "/Expiry",
		Message:   message,
		Path:      path,
		StartLine: int32(expiryLine.LineNum),
		EndLine:   int32(expiryLine.LineNum),
		StartChar: int32(expiryLine.StartIndex),
		EndChar:   int32(expiryLine.EndIndex),
	}
}

func checkEnums(path string, hist *histogram, meta *metadata, singletonEnums stringset.Set) *tricium.Data_Comment {
	if singletonEnums.Has(hist.Enum) && !strings.Contains(hist.Summary, "baseline") {
		enumLine := meta.attributeMap[enumAttribute]
		log.Printf("ADDING Comment for %s at line %d: %s", hist.Name, enumLine.LineNum, "Single Element Enum No Baseline")
		return &tricium.Data_Comment{
			Category:  category + "/Enums",
			Message:   singleElementEnumWarning,
			Path:      path,
			StartLine: int32(enumLine.LineNum),
			EndLine:   int32(enumLine.LineNum),
			StartChar: int32(enumLine.StartIndex),
			EndChar:   int32(enumLine.EndIndex),
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

func generateCommentsForAddedNamespaces(path string, newNamespaces stringset.Set, oldNamespaces stringset.Set, namespaceLineNums map[string]int) []*tricium.Data_Comment {
	var comments []*tricium.Data_Comment
	allAddedNamespaces := newNamespaces.Difference(oldNamespaces).ToSlice()
	sort.Strings(allAddedNamespaces)
	for _, namespace := range allAddedNamespaces {
		comment := &tricium.Data_Comment{
			Category:  category + "/Namespace",
			Message:   fmt.Sprintf(addedNamespaceWarning, namespace),
			Path:      path,
			StartLine: int32(namespaceLineNums[namespace]),
			EndLine:   int32(namespaceLineNums[namespace]),
		}
		log.Printf("ADDING Comment for %s at line %d: %s", namespace, comment.StartLine, "[WARNING]: Added Namespace")
		comments = append(comments, comment)
	}
	return comments
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

func showAllComments(comments []*tricium.Data_Comment) []*tricium.Data_Comment {
	for _, comment := range comments {
		comment.ShowOnUnchangedLines = true
	}
	return comments
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
		if inputDate, _, _, ok := getExpiryDate(hist.Expiry); ok {
			dateDiff := int(inputDate.Sub(now()).Hours() / 24)
			return dateDiff < -1*numDays
		}
	}
	return false
}
