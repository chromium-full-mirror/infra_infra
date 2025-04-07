// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import React from 'react';

import { Typography } from '@mui/material';
import { render } from 'react-dom';
import { CacheProvider } from '@emotion/react';
import createCache, { EmotionCache } from '@emotion/cache';
import { AlertReasonJson, AlertReasonTestJson } from './server_json';
import { DisableTestButton } from './disable_button';
import BugReportIcon from '@mui/icons-material/BugReport';


interface ReasonSectionProps {
    failure_bbid: string;
    tree: string;
    reason: AlertReasonJson | null | undefined;
    bugs: Bug[];
}

interface Bug {
    id: string;
}

const codeSearchLink = (t: AlertReasonTestJson): string => {
    let query = t.test_name;
    if (t.test_name.includes('#')) {
        // Guessing that it's a java test; the format expected is
        // test.package.TestClass#testMethod. For now, just split around the #
        let split = t.test_name.split('#');

        if (split.length > 2) {
            console.error('invalid java test name', t.test_name);
        } else {
            query = split[0] + ' function:' + split[1];
        }
    }
    return `https://cs.chromium.org/search/?q=${encodeURIComponent(query)}`;
}

const historyLink = (t: AlertReasonTestJson): string => {
    const project = encodeURIComponent(t.realm.split(':', 2)[0]);
    const testId = encodeURIComponent(t.test_id);
    const query = encodeURIComponent('VHASH:' + t.variant_hash);
    return `https://ci.chromium.org/ui/test/${project}/${testId}?q=${query}`;
}

interface SimilarFailuresLinkProps {
    test: AlertReasonTestJson
}
const SimilarFailuresLink = ({test}: SimilarFailuresLinkProps) => {
    const [project, algorithm, id] = test.cluster_name.split('/', 3);
    if (algorithm.startsWith('rules')) {
        return <a style={{display: 'flex', alignItems: 'center'}} href={`https://luci-analysis.appspot.com/p/${project}/rules/${id}`} target="_blank"><BugReportIcon />  Bug</a>;
    }
    return <a href={`https://luci-analysis.appspot.com/p/${project}/clusters/${algorithm}/${id}`} target="_blank">Similar Failures</a>;
}

const chromiumTrees = ['chromium', 'chromium.gpu', 'chromium.perf', 'chrome_browser_release'];
const isChromiumTree = (tree: string): boolean => 
    chromiumTrees.indexOf(tree) != -1;

export const ReasonSection = (props: ReasonSectionProps) => {
    if (!props.reason?.tests?.length) {
        return <>No test result data available.</>
    }
    return <>
        <Typography variant='body1' sx={{ color: '#000' }}>
            {props.reason.num_failing_tests} tests failed
            {props.reason.tests.length < props.reason.num_failing_tests ? `, showing ${props.reason.tests.length}` : ''}
            :
        </Typography>
        <table>
            <tbody>
                {props.reason.tests?.map(t => <tr>
                    <td>{t.test_name}</td>
                    <td>{isChromiumTree(props.tree)?<a href={codeSearchLink(t)} target="_blank">Code Search</a>:null}</td>
                    <td><a href={historyLink(t)} target="_blank">History</a></td>
                    <td><SimilarFailuresLink test={t} /></td>
                    <td>{isChromiumTree(props.tree)?<DisableTestButton bugs={props.bugs} testName={t.test_name} testID={t.test_id} failure_bbid={props.failure_bbid} />:null}</td>
                </tr>)}
            </tbody>
        </table>
    </>
}

export class SomReasonSection extends HTMLElement {
    cache: EmotionCache;
    child: HTMLSpanElement;
    props: ReasonSectionProps = {
        tree: '',
        reason: null,
        bugs: [],
        failure_bbid: '',
    };

    constructor() {
        super();
        const root = this.attachShadow({ mode: 'open' });
        const parent = document.createElement('span');
        this.child = document.createElement('span');
        root.appendChild(parent).appendChild(this.child);
        this.cache = createCache({
            key: 'som-reason',
            container: parent,
        });
    }
    connectedCallback() {
        this.render();
    }

    set tree(value: string) {
        this.props.tree = value;
        this.render();
    }

    set reason(value: AlertReasonJson | null | undefined) {
        this.props.reason = value;
        this.render();
    }

    set bugs(value: Bug[]) {
        this.props.bugs = value;
        this.render();
    }

    set failure_bbid(value: string) {
        this.props.failure_bbid = value;
        this.render();
    }

    render() {
        if (!this.isConnected) {
            return;
        }
        render(
            <CacheProvider value={this.cache}>
                <ReasonSection {...this.props} />
            </CacheProvider>,
            this.child
        );
    }
}

customElements.define('som-reason-section', SomReasonSection);