// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import React from 'react';

import CloseIcon from '@mui/icons-material/Close';
import { render } from 'react-dom';
import { CacheProvider } from '@emotion/react';
import  createCache,{ EmotionCache } from '@emotion/cache';
import { obtainAuthState } from './auth_state';
import { QueryClientProvider, useQuery } from 'react-query';
import { queryClient } from './server_json';
import styled from '@emotion/styled';

const GOOGLE_ISSUE_TRACKER_API_ROOT = 'https://issuetracker.corp.googleapis.com';
const GOOGLE_ISSUE_TRACKER_DISCOVERY_PATH = '/$discovery/rest';
const version = 'v3';

interface BugChipProps {
  bug: Bug
  onRemove: () => void
}

interface Bug {
 id: string
 projectId: string
 summary?:string
 status?: string
}


interface IssueTrackerIssue {
  issueState: IssueState
}

interface IssueState {
  title: string
  status: string
  severity: string
}

const BugDiv = styled.div({
  fontSize: '0.8em',
  background: '#eee',
  borderRadius: '16px',
  padding: '1px 2px 1px 4px',
  display: 'inline-block',
  margin: '0 2px',
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  cursor: 'default'
});

const CloseIconStyle = {
  color: '#666',
  padding: '0',
  margin: '0 2px',
  height: '16px',
  width: '16px',
  WebkitTransition: 'all .3s ease',
  transition: 'all .3s ease',
  borderRadius: '50%',
  verticalAlign: 'middle',
  "&:hover": { color: "#222", backgroundColor:"#aaa" }};

export const BugChip = ({bug, onRemove}: BugChipProps) => {
  const isIssueTrackerIssue =  bug.projectId === 'b'

  const bugURL = isIssueTrackerIssue ? `https://issuetracker.google.com/issues/${bug.id}`
                                        :`https://crbug.com/${bug.projectId}/${bug.id}`
  const {data, error} = useQuery(['issuetracker','getIssue', bug.id ],
    async () => {
      if (!isIssueTrackerIssue){
        return null
      }
      const authState = await obtainAuthState()
      gapi.client.setToken({access_token:authState.accessToken})
      // Load the issue tracker interface if not yet loaded.
      if (!gapi.client.corp_issuetracker){
        await gapi.client.load(
          GOOGLE_ISSUE_TRACKER_API_ROOT + GOOGLE_ISSUE_TRACKER_DISCOVERY_PATH,
          version)
      }
      return await gapi.client.corp_issuetracker.issues.get({'issueId': bug.id })
    }
  )
  const issue: IssueTrackerIssue = data?.result

  return <>
    <BugDiv className="bug no-toggle">
            <a target="_blank" href={bugURL}>Bug {bug.id}</a>
            {" "}
            {isIssueTrackerIssue?
              // Buganizer issue.
              (!error && issue && (
                <>
                  { issue.issueState.title || "" }
                  {issue.issueState.status && (<em className="bug-status" >{`(${issue.issueState.status})`}</em>)}
                </>
              ))
            :(
              // Monorail issue.
              <>
              {bug.summary || ""}
              {bug.status && (<em className="bug-status" >{`(${bug.status})`}</em>)}
              </>
            )}
          <CloseIcon sx={CloseIconStyle} onClick={onRemove}/>
    </BugDiv>
  </>
}

export class SomBugChip extends HTMLElement {
  cache: EmotionCache;
  child: HTMLDivElement;
  props: BugChipProps = {
    bug: {
      id: "",
      projectId: ""
    },
    onRemove: ()=> {}
  };

  constructor() {
    super();
    const root = this.attachShadow({ mode: 'open' });
    const parent = document.createElement('div');
    this.child = document.createElement('div');
    root.appendChild(parent).appendChild(this.child);
    this.cache = createCache({
        key: 'bug-chip',
        container: parent,
    });
}

  connectedCallback() {
    this.render();
  }

  set bug(bug: Bug) {
    this.props.bug = bug;
    this.render();
  }

  _removeBug() {
    let bug = this.props.bug
    this.dispatchEvent(new CustomEvent('remove-pressed', {
      detail: {
        bug: String(bug.id),
        summary: bug.summary,
        project: String(bug.projectId),
        url: 'https://crbug.com/' + bug.projectId + '/' + bug.id,
      },
      bubbles: true,
      composed: true,
    }));
  }

  render() {
    if (!this.isConnected) {
      return;
    }
    render(
      <CacheProvider value={this.cache}>
          <QueryClientProvider client={queryClient}>
            <BugChip bug={this.props.bug} onRemove={this._removeBug.bind(this)} />
          </QueryClientProvider>
        </CacheProvider>
    ,this.child);
  }
}

customElements.define('som-bug-chip', SomBugChip);