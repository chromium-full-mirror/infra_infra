// Copyright 2020 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package eval

import (
	"fmt"
	"io"
	"math"
	"time"

	"go.chromium.org/luci/common/data/text/indented"

	evalpb "go.chromium.org/infra/rts/presubmit/eval/proto"
)

type printer struct {
	indented.Writer
	err error
}

func newPrinter(w io.Writer) *printer {
	return &printer{
		Writer: indented.Writer{
			Writer:    w,
			UseSpaces: true,
			Width:     2,
		},
	}
}

func (p *printer) printf(format string, args ...interface{}) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(&p.Writer, format, args...)
	}
}

// psURL returns the patchset URL.
func psURL(p *evalpb.GerritPatchset) string {
	return fmt.Sprintf("https://%s/c/%d/%d", p.Change.Host, p.Change.Number, p.Patchset)
}

// PrintResults prints the results to w.
func PrintResults(res *evalpb.Results, w io.Writer, minChangeRecall float32) error {
	return PrintSpecificResults(res, w, minChangeRecall, true, true)
}

func PrintSpecificResults(res *evalpb.Results, w io.Writer, minChangeRecall float32, printTestRecall bool, printDistance bool) error {
	p := newPrinter(w)

	p.printf("ChangeRecall | Savings")
	if printTestRecall {
		p.printf(" | TestRecall")
	}
	if printDistance {
		p.printf(" | Distance")
	}
	p.printf("\n")
	p.printf("----------------------")
	if printTestRecall {
		p.printf("-------------")
	}
	if printDistance {
		p.printf("-----------")
	}
	p.printf("\n")

	for i, t := range res.Thresholds {
		if t.ChangeRecall < minChangeRecall || (i > 0 &&
			res.Thresholds[i].ChangeRecall == res.Thresholds[i-1].ChangeRecall &&
			res.Thresholds[i].Savings == res.Thresholds[i-1].Savings &&
			res.Thresholds[i].TestRecall == res.Thresholds[i-1].TestRecall) {
			continue
		}
		p.printf(
			"%7s      | % 7s ",
			scoreString(t.ChangeRecall),
			scoreString(t.Savings),
		)
		if printTestRecall {
			p.printf(
				"| %7s    ",
				scoreString(t.TestRecall),
			)
		}
		if printDistance {
			p.printf(
				"| %6.3f",
				t.MaxDistance,
			)
		}
		p.printf("\n")
	}
	p.printf("\nbased on %d rejections, %d test failures, %s testing time\n", res.TotalRejections, res.TotalTestFailures, durationString(res.TotalDuration.AsDuration()))
	return p.err
}

func durationString(duration time.Duration) string {
	seconds := int64(duration.Seconds())
	minutes := seconds / 60
	if minutes == 0 {
		return fmt.Sprintf("%d seconds", seconds)
	}
	seconds = seconds % 60
	hours := minutes / 60
	if hours == 0 {
		return fmt.Sprintf("%d minutes %d seconds", minutes, seconds)
	}
	minutes = minutes % 60
	days := hours / 24
	if days == 0 {
		return fmt.Sprintf("%d hours %d minutes %d seconds", hours, minutes, seconds)
	}
	hours = hours % 24
	years := days / 365
	if years == 0 {
		return fmt.Sprintf("%d days %d hours %d minutes %d seconds", days, hours, minutes, seconds)
	}
	days = days % 365
	return fmt.Sprintf("%d years %d days %d hours %d minutes %d seconds", years, days, hours, minutes, seconds)
}

func scoreString(score float32) string {
	percentage := score * 100
	switch {
	case math.IsNaN(float64(percentage)):
		return "?"
	case percentage > 0 && percentage < 0.01:
		// Do not print it as 0.00%.
		return "<0.01%"
	case percentage > 99.99 && percentage < 100:
		// Do not print it as 100.00%.
		return ">99.99%"
	default:
		return fmt.Sprintf("%02.2f%%", percentage)
	}
}
