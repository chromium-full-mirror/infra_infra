# RTS test mapping analysis CLI

This CLI makes use of the evaluation built into the RTS framework to analyze
the expected results of using a test similarity json file to analyze the
impact of remove several test cases.

The json file is structured as {test_id_1: [test_id_2, test_id_3]} based on the
test mapping clustering result.

Before attempting to build, env.py should be run as described in
infra/go/README.md

You will also need IAM permissions for the `chrome-rts` project.

1. Fetch a sample of rejections, e.g. for the past month of `browser_tests` on builder `linux-rel`.

   ```bash
   go run ./cmd/rts-test-mapping fetch-rejections \
     -from 2024-08-01 -to 2024-08-31 \
     -builder linux-rel -test_suite browser_tests \
     -out samples.rej
   ```

1. Fetch a sample of test durations, e.g. for for the past month of `browser_tests` on builder `linux-rel`.

   ```bash
   go run ./cmd/rts-test-mapping fetch-durations \
     -from 2024-08-01 -to 2024-08-31 \
     -builder linux-rel -test_suite browser_tests \
     -out samples.dur
   ```

   Note: These records are expensive to collect/analyze. To reduce this cost
   only a fraction of the applicable durations will be collected. By default
   this optional `-frac` flag hold .1 (10%) over the given time period and savings
   are estimated based this fraction (the durations don't need to be over the
   same time period as rejections to function as this is an estimate, as long
   as the average test in the suite hasn't changed dramatically the estimate
   should still be accurate)

   `-ignore-file` is necessary for any builder/test suite that doesn't include
   its filename when uploaded to rdb. This should be passed when using
   rts-test-mapping.

   The savings provided later by this tool is based on all durations gathered
   in this step. If you're more curious about savings for a particular builder
   or set of builders you can provide the `-builder` arg with a regex to match
   specific builders

1. Run the analysis on the rejections and durations which you created from -out
   in the previous steps, note the ChangeRecall/Savings:

   ```bash
   go run ./cmd/rts-test-mapping analyze \
     -rejections samples.rej \
     -durations samples.dur \
     -builder linux-rel \
     -similarity_json test_mapping.json
   ```

   Output will look something like:

   ```
   ChangeRecall | Savings
   ----------------------
    99.78%      |  14.45%

   based on 6878 rejections, 8066798 test failures, 913690h51m52.584049s testing time
   ```

   The ChangeRecall being the expected number of failed CLs in the given time
   period that would still have failed if the test cases were not run on the
   builder (successful CLs do not factor into this number). The savings is how
   much estimated time would have been saved for that tradeoff in recall. For
   the period of time the rejections were collected the ChangeRecall percentage
   is the number that would still have been caught. The rate can then be
   estimated as (1 - ChangeRecall) * rejections per period of reject collection.
   For example, the output above shows that there were 6878 rejections in August
   2024. Of these, 99.78% of those rejections would have been preserved. This
   means that running this test cluster based strategy from `linux-rel`
   would have allowed .22% of rejections through, or about 15 CLs over the
   course of a month. (6878 rejections * .22% rejections in August).

   Also note the number of rejections as this will match the number of
   rejections collected in the fetch-rejections step and can help confirm the
   values are reasonable.

   Example rejects will be printed after the run and before the previous table.
   For example:

   ```1 furthest rejections:
    Rejection:
      Most affected test: +Inf distance
      https://chromium-review.googlesource.com/c/4080905/1
        //some/file.cc
        more files...
        //these/can/be/ignored/for/rts-analysis.c
      Failed and not selected tests:
        - builder:linux_chromium_asan_rel_ng | os:linux | test_suite:browser_tests
          in <unknown file>
            ninja://test/case/unique/id
   ```

   Up to 10 missed rejects will be printed by default. The "Most affected test"
   and changed files for each reject does not have any meaning with this
   application of the evaluation and can be ignored. If the recall is very high
   this list can be very short. It is a good idea to investigate these using
   the provided link since the analysis cannot know which rejects were a false
   reject and these failures can be builder flakes where it should not
   have been rejected in the first place.

Rejection and duration data can take a long time to collect (you will likely not
want to go over a months worth of data as the results tend to stay consistent
while the collection time can be very long) but are reusable for testing
different builder/test suite combinations and only the last step needs to be
repeated for different tests.

To avoid having to fetch all the durations and rejections for a long period
of time the flag -append can be added to the command. This will add the new
records from this run to any existing data in the out dir. Caution needs to
be taken to make sure that a day is not fetched twice. These rejections can be
found in the output directory. To remove old data from the records you can
directly remove the .jsonl.gz files with the "from" and "to" prepended to the
file.

After a builder has a specific test suite removed it invalidates the rejection
and duration data since there could have been overlapping coverage of tests. To
perform a new analysis on the same rejection/duration data or time period all
removed builder/suite combinations should be included in a file in the format:

builder:test_suite

and added to a file as a new line. That file's name can then be included like:

   ```bash
   go run ./cmd/rts-test-mapping analyze \
     -rejections samples.rej \
     -durations samples.dur \
     -builder linux_chromium_asan_rel_ng \
     -similarity_json test_mapping.json
   ```
