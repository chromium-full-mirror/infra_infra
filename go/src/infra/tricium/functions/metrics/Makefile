# Copyright 2019 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

.PHONY: test unit-test clean

metrics_analyzer:
	go build -o metrics_analyzer

clean:
	rm -rf out metrics_analyzer

test: clean metrics_analyzer
	./metrics_analyzer -input=testdata/src -output=out/tricium_test_diff -previous=prevdata/src -patch=prevdata/tricium_test_diff.patch -enums=enums/enums.xml -message='commit message.\n\nOBSOLETE_HISTOGRAM[Histogram.Foo]=typo\nBug: 124343' -- test/histograms.xml
	./metrics_analyzer -input=testdata/src -output=out/add_new_suffix_diff -previous=prevdata/src -patch=prevdata/add_new_suffix_diff.patch -enums=enums/enums.xml -message='commit message\n Bug: 12345' -- suffixes/histogram_suffixes_list.xml
	./metrics_analyzer -input=testdata/src -output=out/patterned_histogram_diff -previous=prevdata/src -patch=prevdata/tricium_patterned_histogram_diff.patch -enums=enums/enums.xml -message='commit message\n\nOBSOLETE_HISTOGRAM[Test.Bulbasaur.Histogram]=typo\nBug: 12345' -- test/histograms.xml rm/histograms.xml
	./metrics_analyzer -input=testdata/src -output=out/global_message_diff -previous=prevdata/src -patch=prevdata/tricium_patterned_histogram_diff.patch -enums=enums/enums.xml -message='commit message\n\nOBSOLETE_HISTOGRAMS=typo\nOBSOLETE_HISTOGRAM[Test.Bulbasaur.Histogram]=typo\nBug: 12345' -- test/histograms.xml rm/histograms.xml

unit-test:
	go test histograms_check_test.go field_trial_check_test.go metrics_analyzer.go histograms_check.go field_trial_check.go
