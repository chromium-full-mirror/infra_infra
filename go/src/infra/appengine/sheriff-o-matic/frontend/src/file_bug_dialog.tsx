// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import React, { useState } from 'react';

import { Alert, Button, CircularProgress, Dialog, DialogActions, DialogContent, DialogTitle, Link, TextField, Typography } from '@mui/material';
import { render } from 'react-dom';
import { useMutation, QueryClientProvider } from 'react-query';

import { AlertBuilderJson, AlertJson, queryClient, TreeJson, treeJsonFromName } from './server_json';
import { isTrooperAlertType } from './alert_types';

interface FileBugDialogProps {
    alerts: AlertJson[];
    tree: TreeJson;
    open: boolean;
    onClose: () => void;
}

interface linkBugMutationData {
    bugId: string;
}

export const FileBugDialog = (props: FileBugDialogProps) => {
    const [bugId, setBugId] = useState('');
    const linkBugMutation = useMutation((data: linkBugMutationData): Promise<void> => {
        return linkBug(props.tree, props.alerts, data.bugId);
    }, {
        onSuccess: () => props.onClose(),
    });
    if (!props.tree) {
        return null;
    }
    if (linkBugMutation.isLoading) {
        <Dialog open={props.open} onClose={props.onClose}>
            <DialogTitle>File a new bug</DialogTitle>
            <CircularProgress></CircularProgress>
        </Dialog>
    }

    return <Dialog open={props.open} onClose={props.onClose}>
        <DialogTitle>File a new bug</DialogTitle>
        <DialogContent>
            {linkBugMutation.isError ? <Alert severity='error'>Error linking bug: {(linkBugMutation.error as Error).message}</Alert> : null}
            <Typography>Please use <Link target="_blank" href={fileBugLink(props.tree, props.alerts)}>this link to create a bug</Link> for this alert.</Typography>
            <Typography>Once it is created, please copy the bug id into the box to link the bug.</Typography>
            <TextField sx={{ marginTop: '10px' }} label="Bug ID" autoFocus value={bugId} onChange={e => setBugId(e.target.value)} />
        </DialogContent>
        <DialogActions>
            <Button onClick={() => props.onClose()}>Close</Button>
            <Button onClick={() => linkBugMutation.mutate({ bugId })}>Link Bug</Button>
        </DialogActions>
    </Dialog>;
}

const monorailProjectId = (tree: TreeJson): string => {
    return tree.default_monorail_project_name || 'chromium';
}
const fileBugLink = (tree: TreeJson, alerts: AlertJson[]): string => {
    const description = bugComment(tree, alerts);
    const summary = bugSummary(tree, alerts);
    if (tree.hotlist_id) {
        return `https://b.corp.google.com/issues/new?title=${encodeURIComponent(summary)}&description=${encodeURIComponent(description)}&hotlistIds=${encodeURIComponent(tree.hotlist_id)}`;
    }
    const projectId = monorailProjectId(tree);
    const labels = bugLabels(tree, alerts);
    return `https://bugs.chromium.org/p/${encodeURIComponent(projectId)}/issues/entry?summary=${encodeURIComponent(summary)}&description=${encodeURIComponent(description)}&labels=${encodeURIComponent(labels.join(','))}`;
}

const bugSummary = (tree: TreeJson, alerts: AlertJson[]): string => {
    let bugSummary = 'Bug filed from Sheriff-o-Matic';
    if (alerts && alerts.length) {
        if (tree.name === 'android' && alerts[0].extension && alerts[0].extension.builders &&
            alerts[0].extension.builders.length === 1 && alertIsTestFailure(alerts[0])) {
            bugSummary = `<insert test name/suite> is failing on builder "${alerts[0].extension.builders[0].name}"`;
        } else {
            bugSummary = alerts[0].title;
        }
        if (alerts.length > 1) {
            bugSummary += ` and ${alerts.length - 1} other alerts`;
        }
    }
    return bugSummary;
}

const alertIsTestFailure = (alert: AlertJson): boolean => {
    return alert.type === 'test-failure' ||
        (alert.extension && alert.extension.reason && alert.extension.reason.step
            && alert.extension.reason.step.includes('test'));
}

const bugComment = (tree: TreeJson, alerts: AlertJson[]): string => {
    return alerts.reduce((comment, alert) => {
        let result = '';
        if (alert.extension && alert.extension.builders && alert.extension.builders.length > 0) {
            const isTestFailure = alertIsTestFailure(alert);
            if (alert.extension.builders.length === 1 && isTestFailure && tree.name === 'android') {
                result += `<insert test name/suite> is failing in step "${alert.extension.reason.step}" on builder "${alert.extension.builders[0].name}"\n\n`;
            } else {
                result += alert.title + '\n\n';
            }
            const failuresInfo = [];
            for (const builder of alert.extension.builders) {
                failuresInfo.push(builderFailureInfo(builder));
            }
            result += 'List of failed builders:\n\n' +
                failuresInfo.join('\n--------------------\n') + '\n';

            if (tree.name === 'android') {
                result += `
------- Note to sheriffs -------

For failing tests:
Please file a separate bug for each failing test suite, filling in the name of the test or suite (<in angle brackets>).

Add a component so that bugs end up in the appropriate triage queue, and assign an owner if possible.

If applicable, also include a sample stack trace, link to the flakiness dashboard, and/or post-test screenshot to help with future debugging.

If a culprit CL can be identified, revert the CL. Otherwise, disable the test.
When either action is complete and the issue no longer requires sheriff attention, remove the ${tree.bug_queue_label} label.

For infra failures:
See go/bugatrooper for instructions and bug templates

------------------------------
`;
            }
        }
        return comment + result;
    }, '');
}

const builderFailureInfo = (builder: AlertBuilderJson): string => {
    let s = 'Builder: ' + builder.name;
    s += '\n' + builder.url;
    if (builder.first_failure_url) {
        s += '\n' +
            'First failing build:';
        s += '\n' + builder.first_failure_url;
    } else if (builder.latest_failure_url) {
        s += '\n' +
            'Latest failing build:';
        s += '\n' + builder.latest_failure_url;
    }
    return s;
}

const bugLabels = (tree: TreeJson, alerts: AlertJson[]): string[] => {
    const labels = ['Filed-Via-SoM'];
    if (!tree) {
        return labels;
    }

    if (tree.name === 'android') {
        labels.push('Restrict-View-Google');
    }
    if (tree.bug_queue_label) {
        labels.push(tree.bug_queue_label);
    }

    if (alerts) {
        const trooperBug = alerts.some((alert) => {
            return isTrooperAlertType(alert.type);
        });

        if (trooperBug) {
            labels.push('Infra-Troopers');
        }
    }
    return labels;
}

const linkBug = async (tree: TreeJson, alerts: AlertJson[], bugId: string): Promise<void> => {
    const data = {
        bugs: [{
            id: bugId,
            projectId: monorailProjectId(tree),
        }],
    };

    const promises = alerts.map((alert) => post('/api/v1/annotations/' + tree.name + '/add', { ...data, key: alert.key }));
    await Promise.all(promises);
}
const post = async (path: string, data: any): Promise<any> => {
    const response = await fetch(path, {
        method: 'POST',
        credentials: 'include',
        body: JSON.stringify({
            xsrf_token: (window as any).xsrfToken,
            data,
          }),
    })
    if (!response.ok) {
        throw new Error(await response.text())
    }
    return response.json()
}

export class SomFileBugDialog extends HTMLElement {
    props: FileBugDialogProps = {
        alerts: [],
        tree: null,
        open: false,
        onClose: () => { },
    };

    _onCloseHandler?: () => void;

    connectedCallback() {
        this.props.onClose = () => {
            this.props.open = false;
            this.render();
        }
        this.render();
    }

    open(treeName: string, alerts: AlertJson[]) {
        this.props.open = true;
        this.props.tree = treeJsonFromName(treeName);
        this.props.alerts = alerts;
        this.render();
    }

    render() {
        if (!this.isConnected) {
            return;
        }
        render(<QueryClientProvider client={queryClient}>
            <FileBugDialog {...this.props} />
        </QueryClientProvider>, this);
    }
}

customElements.define('som-file-bug-dialog', SomFileBugDialog);