# Manual Testing Guide for `shivas repair-duts`

This guide outlines the steps for manually testing the `shivas repair-duts` command.

## Prerequisites

*   A locally built `shivas` executable
*   A test environment configured to point to correct UFS environment.
*   A working local authentication setup to interact with Buildbucket and UFS.

## Testing Steps

Follow these steps to perform manual testing:

1.  **Build `shivas`:** Build the `shivas` executable locally. Take a look at the makefile under `shivas` directory for more info
2.  **Configure Environment:** Ensure your test environment is configured correctly, including pointing to the correct UFS instance and setting up authentication (take a look at `site/site.go`).
3.  **Run Basic Test Cases:** Execute the `shivas repair-duts` command from your terminal for various scenarios without the `--admin-lib` flag. Examples of scenarios to test include:
    *   Running with no DUT hostnames (expect error).
    *   Running without proper authentication (expect error).
    *   Scheduling tasks for a single valid DUT hostname (default flags).
    *   Scheduling tasks for multiple valid DUT hostnames (default flags).
    *   Using the `--verify` flag.
    *   Using the `--deep` flag.
    *   Using the `--latest` flag.
    *   Specifying a custom `--bucket`.
    *   Specifying a custom `--builder`.
    *   Combining `--verify` and `--deep` flags.
    *   Handling invalid or non-existent DUT hostnames.
4.  **Test with Admin Library (Optional):** To test the new shared admin library implementation, repeat the test cases from step 3, but include the `--admin-lib=true` flag.
5.  **Observe and Record Results:** For each test case, observe the command output (e.g., task URLs, error messages) and record the results.
6.  **Measure Performance (Optional):** Use the `time` command to measure the execution time for single and multiple DUT scenarios, both with and without the `--admin-lib` flag.
7.  **Compare Results:** Compare the observed results (task scheduling, error handling) with the expected behavior. If testing with `--admin-lib=true`, compare those results with the results from running the command without the flag.

For detailed results of specific test cases, please refer to the full document: [https://docs.google.com/document/d/1ZwsPXvXZTerT1F_oS8eR6WY9qsIcO9yCC6xHz4FVIc8/edit?tab=t.0#heading=h.e3k0jnuon3nx](https://docs.google.com/document/d/1ZwsPXvXZTerT1F_oS8eR6WY9qsIcO9yCC6xHz4FVIc8/edit?tab=t.0#heading=h.e3k0jnuon3nx)