// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import React, { useMemo } from 'react';
import { Box, Typography, Link, Chip, Stack } from '@mui/material';
import { CacheProvider } from '@emotion/react';
import createCache, { EmotionCache } from '@emotion/cache';
import { render } from 'react-dom';

import { ReasonSection } from './reason_section';
import {
  LuciBisectionResultSection,
  LuciBisectionResult
} from './luci_bisection_result';
import { RevRange, } from './rev_range';
import { AlertBuilderJson, Bug, AlertReasonJson, AlertReasonTestJson, RegressionRangeJson } from './server_json';

interface ExtensionJson {
  builders?: AlertBuilderJson[];
  reason?: AlertReasonJson | null;
  regression_ranges?: RegressionRangeJson[];
  luci_bisection_result?: LuciBisectionResult | null;
}

const getFailureCount = (builder: AlertBuilderJson): number => {
  // The build number range is inclusive.
  return builder.latest_failure_build_number - builder.first_failure_build_number + 1;
};

const getFailureCountText = (builder: AlertBuilderJson): string => {
  const numBuilds = getFailureCount(builder);

  // first_failure_build_number == 0 means we do not have information about
  // the first failure. In this case, we do not display anything.
  if (numBuilds <= 1 || builder.first_failure_build_number === 0) {
    return '';
  }

  if (builder.count) {
    return `[${builder.count} out of the last ${numBuilds} builds have failed]`;
  }

  return `[${numBuilds} since first detection]`;
};

const getDisplayName = (builder: AlertBuilderJson, tree: string): string => {
  if (tree === 'chrome_browser_release') {
    return `${builder.project}.${builder.name}`;
  }
  return builder.name;
};

const getFailureBbid = (extension: ExtensionJson | null | undefined): string => {
  const builders = extension?.builders;
  if (!builders || builders.length === 0) {
    return '';
  }
  // We cannot use the latest_failure directly due to crbug.com/1366166
  // So we need to work around
  const url = builders[0].latest_failure_url;
  if (!url) {
    return "";
  }
  // url is of the form https://ci.chromium.org/.../b<bbid>
  const lastBIndex = url.lastIndexOf("/b");
  return lastBIndex !== -1 ? url.substring(lastBIndex + 2) : "";
}

const getChipStyles = (builder: AlertBuilderJson, type: string | undefined, failureCount: number) => {
  const isInfra = type === 'infra-failure' || builder.build_status === "INFRA_FAILURE";
  const hasMultipleFailures = failureCount > 1; // Assuming count based on range

  return {
    mr: 0.5,
    mb: 0.5,
    fontWeight: 'bold',
    color: '#fff', // Default text color
    bgcolor: isInfra ? '#e0b0ff' : '#e75d54', // Infra or normal failure background
    border: '1px solid #dcdcdc',
    '& .MuiChip-label': {
      paddingLeft: '0.5em',
      paddingRight: '0.5em',
    },
    '&:hover': {
      textDecoration: 'underline',
      bgcolor: isInfra ? '#e0b0ff' : '#e75d54',
    },
    ...(hasMultipleFailures && {
    }),
  };
};

interface ExtensionBuildFailureProps {
  extension: ExtensionJson | null | undefined;
  type?: string;
  tree: string;
  bugs: Bug[];
}

export const ExtensionBuildFailure: React.FC<ExtensionBuildFailureProps> = ({
  extension,
  type,
  tree,
  bugs = [],
}) => {

  const processedReason = useMemo(() => {
    // De-duplication logic (keep from previous step)
    if (!extension?.reason?.tests) return extension?.reason;
    const seen = new Set<string>();
    const uniqueTests = extension.reason.tests.filter((test: AlertReasonTestJson) => {
      if (seen.has(test.test_name)) return false;
      seen.add(test.test_name);
      return true;
    });
    return { ...extension.reason, tests: uniqueTests };
  }, [extension?.reason]);

  const failureBbid = useMemo(() => getFailureBbid(extension), [extension?.builders]);
  const haveBuilders = extension?.builders && extension.builders.length > 0;
  const haveRegressionRanges = extension?.regression_ranges && extension.regression_ranges.length > 0;
  const haveLuciBisectionResult = !!extension?.luci_bisection_result;

  // Filter valid regression ranges (replicating _showRegressionRange filter)
  const validRegressionRanges = useMemo(() => {
    // Filter out ranges with errors for rendering purposes
    return extension?.regression_ranges?.filter(range => !!range && !range.error) ?? [];
  }, [extension?.regression_ranges]);


  if (!extension) {
    return <Box><Typography>Loading extension data...</Typography></Box>;
  }

  return (
    <Box>
      {haveBuilders && (
        <Box id="builders" sx={{ mb: 1.5 }}>
          <Typography variant="body1" sx={{ color: '#000', mb: 0.5 }}>
            Builders this step failed on:
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap' }}>
            {extension.builders?.map((builder, index) => {
              const failureCount = getFailureCount(builder);
              const title = `Failing for the last ${failureCount} build(s): From build ${builder.first_failure_build_number} to build ${builder.latest_failure_build_number}.`;
              const label = `${getDisplayName(builder, tree)} ${getFailureCountText(builder)}`;

              return (
                <Chip
                  key={builder.url || index}
                  label={label.trim()}
                  component="a"
                  href={builder.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  clickable
                  title={title}
                  sx={getChipStyles(builder, type, failureCount)}
                />
              );
            })}
          </Box>
        </Box>
      )}

      <Box className="section" sx={{ pb: { xs: 2, lastOfType: 0 } }}>
        <ReasonSection
          tree={tree}
          bugs={bugs}
          reason={processedReason}
          failure_bbid={failureBbid}
        />
      </Box>

      <Box id="regressionRanges" className="section" sx={{ mb: 1.5, pb: { xs: 2, '&:last-of-type': 0 } }}>
        {!haveRegressionRanges ? (
          <Typography>No regression range information available.</Typography>
        ) : (
          validRegressionRanges.map((range, index) => (
            <RevRange key={index} range={range} />
          ))
        )}
        {extension?.regression_ranges?.some(r => r?.error) && (
          <Typography color="error" sx={{ mt: 1 }}>Note: Some regression range data could not be loaded.</Typography>
        )}
      </Box>

      {haveLuciBisectionResult && extension.luci_bisection_result && (
        <LuciBisectionResultSection result={extension.luci_bisection_result} />
      )}
    </Box>
  );
};

export class SomExtensionBuildFailure extends HTMLElement {
  private mountPoint: HTMLSpanElement;
  private emotionCache: EmotionCache;
  private props: ExtensionBuildFailureProps = {
    extension: null,
    type: '',
    tree: '',
    bugs: [],
  };

  constructor() {
    super();
    const shadowRoot = this.attachShadow({ mode: 'open' });
    const styleContainer = document.createElement('span'); // For Emotion styles
    this.mountPoint = document.createElement('span'); // Where React component mounts
    shadowRoot.appendChild(styleContainer);
    shadowRoot.appendChild(this.mountPoint);

    this.emotionCache = createCache({
      key: 'som-ext-build-failure', // Unique key for styles
      container: styleContainer, // Inject styles into the shadow DOM
    });
  }

  connectedCallback() {
    this.render();
  }

  disconnectedCallback() {
    render(<></>, this.mountPoint);
  }

  static get observedAttributes() {
    return ['type', 'tree'];
  }

  attributeChangedCallback(name: string, oldValue: string | null, newValue: string | null) {
    if (oldValue !== newValue) {
      switch (name) {
        case 'type':
          this.props.type = newValue ?? '';
          break;
        case 'tree':
          this.props.tree = newValue ?? '';
          break;
      }
      this.render();
    }
  }

  set extension(value: ExtensionJson | null | undefined) {
    if (this.props.extension !== value) {
      this.props.extension = value;
      this.render();
    }
  }

  get extension(): ExtensionJson | null | undefined {
    return this.props.extension;
  }

  set type(value: string) {
    const strValue = String(value);
    if (this.props.type !== strValue) {
      this.props.type = strValue;
      this.render();
    }
  }

  get type(): string {
    return this.props.type ?? '';
  }


  set tree(value: string) {
    const strValue = String(value);
    if (this.props.tree !== strValue) {
      this.props.tree = strValue;
      this.render();
    }
  }

  get tree(): string {
    return this.props.tree ?? '';
  }

  set bugs(value: Bug[]) {
    // Basic check; deep equality could be complex/expensive
    if (this.props.bugs !== value) {
      this.props.bugs = Array.isArray(value) ? value : [];
      this.render();
    }
  }

  get bugs(): Bug[] {
    return this.props.bugs;
  }


  render() {
    if (!this.isConnected) {
      return;
    }

    render(
      <CacheProvider value={this.emotionCache}>
        <ExtensionBuildFailure {...this.props} />
      </CacheProvider>,
      this.mountPoint
    );
  }
}

// Register the custom element
customElements.define('som-extension-build-failure', SomExtensionBuildFailure);
