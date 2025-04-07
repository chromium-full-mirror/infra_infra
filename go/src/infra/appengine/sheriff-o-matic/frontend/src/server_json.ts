// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import { QueryClient } from "react-query";

export type TreesJson = TreeJson[];

export interface TreeJson {
    name: string;
    display_name: string;
    bb_project_filter: string;
    default_monorail_project_name?: string;  // default to 'chromium' if undefined.
    bug_queue_label: string;
    // The buganizer hotlist to use for SOM bugs in this tree.
    hotlist_id?: string
}

export const treeJsonFromName = (treeName: string): TreeJson | null => {
    // trees-json is a string containing json, so we need one parse to get the string, and another to get the structure.
    const text = document.getElementById('trees-json')?.innerText;
    if (!text) return null;
    const trees = JSON.parse(JSON.parse(text));
    return trees.filter((t: TreeJson) => t.name == treeName)?.[0];
}

// TODO: AlertJson fields were added based on example data.  There may be missing or incorrect fields.
export interface AlertJson {
    key: string;
    title: string;
    body: string;
    severity: number;
    time: number;
    start_time: number;
    links: null;
    tags: null;
    type: string;
    extension: AlertExtensionJson;
    resolved: boolean;
}

// TODO: AlertExtensionJson fields were added based on example data.  There may be missing or incorrect fields.
export interface AlertExtensionJson {
    builders: AlertBuilderJson[];
    culprits: null;
    has_findings: boolean;
    is_finished: boolean;
    is_supported: boolean;
    reason: AlertReasonJson;
    regression_ranges: RegressionRangeJson[];
    suspected_cls: null;
    tree_closer: false;
}

// TODO: AlertBuilderJson fields were added based on example data.  There may be missing or incorrect fields.
export interface AlertBuilderJson {
    bucket: string;
    build_status: string;
    builder_group: string;
    count: number;
    failing_tests_trunc: string,
    first_failing_rev: RevisionJson;
    first_failure: number;
    first_failure_build_number: number;
    first_failure_url: string;
    last_passing_rev: RevisionJson;
    latest_failure: number;
    latest_failure_build_number: number;
    latest_failure_url: string;
    latest_passing: number;
    name: string;
    project: string;
    start_time: number;
    url: string;
}

// TODO: RevisionJson fields were added based on example data.  There may be missing or incorrect fields.
export interface RevisionJson {
    author: string;
    branch: string;
    commit_position: number;
    description: string;
    git_hash: string;
    host: string;
    link: string;
    repo: string;
    when: number;
}

// TODO: AlertReasonJson fields were added based on example data.  There may be missing or incorrect fields.
export interface AlertReasonJson {
    num_failing_tests: number;
    step: string;
    tests: AlertReasonTestJson[];
}

// TODO: AlertReasonTestJson fields were added based on example data.  There may be missing or incorrect fields.
export interface AlertReasonTestJson {
    test_name: string;
    test_id: string;
    realm: string;
    variant_hash: string;
    cluster_name: string;
}

export interface RegressionRangeJson {
    repo: string;
    host?: string; // Optional based on _shouldUseRangeRevisions logic
    revisions?: string[]; // Commit hashes
    positions?: string[]; // Commit positions eg 'base/stable@{#12345}'
    error?: any; // Indicates an error occurred fetching this range
}

// Placeholder type for bug data
export interface Bug {
    id: string;
}


// Create a React Query client to share globally.
// TODO: once a full conversion to React is complete, move this to the file with the main App component.
export const queryClient = new QueryClient()