import React from 'react';

import { render } from 'react-dom';
import { CacheProvider } from '@emotion/react';
import createCache, { EmotionCache } from '@emotion/cache';
import Box from '@mui/material/Box';
import IconButton from '@mui/material/IconButton';
import HelpOutline from '@mui/icons-material/HelpOutline';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Link from '@mui/material/Link';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';

interface LuciBisectionResultSectionProps {
  result: LuciBisectionResult | null;
}

interface LuciBisectionResult {
  analysis: LuciBisectionAnalysis;
  is_supported: boolean;
  failed_bbid: string;
}

interface LuciBisectionAnalysis {
  analysis_id: string;
  heuristic_result: HeuristicAnalysis;
  nth_section_result: NthSectionAnalysis;
  culprits: Culprit[];
}

interface HeuristicAnalysis {
  suspects: HeuristicSuspect[];
}

interface HeuristicSuspect {
  // TODO (nqmtuan): Also display if a verification is in progress.
  reviewUrl: string;
  justification: string;
  score: number;
  confidence_level: number;
}

interface NthSectionAnalysis {
  suspect: NthSectionSuspect;
  remaining_nth_section_range: RegressionRange;
}

interface NthSectionSuspect {
  reviewUrl: string;
  reviewTitle: string;
}

interface RegressionRange {
  last_passed: GitilesCommit;
  first_failed: GitilesCommit;
}

interface GitilesCommit {
  host: string;
  project: string;
  ref: string;
  id: string;
}

interface Culprit {
  review_url: string;
  review_title: string;
}

interface LuciBisectionCulpritSectionProps {
  culprits: Culprit[];
  failed_bbid: string;
}

export const LuciBisectionCulpritSection = ({
  culprits,
  failed_bbid,
}: LuciBisectionCulpritSectionProps) => {
  return (
    <>
      <Link
        href={generateAnalysisUrl(failed_bbid)}
        target="_blank"
        rel="noopener"
      >
        LUCI Bisection
      </Link>
      &nbsp; has identified the following CL(s) as culprit(s) of the failure:
      <List>
        {culprits.map((c) => (
          <ListItem>
            <Link
              href={c.review_url}
              target="_blank"
              rel="noopener"
              onClick={() => {
                ga('send', {
                  hitType: 'event',
                  eventCategory: 'LuciBisection',
                  eventAction: 'ClickCulpritLink',
                  eventLabel: c.review_url,
                  transport: 'beacon',
                });
              }}
            >
              {getCulpritDisplayUrl(c)}
            </Link>
          </ListItem>
        ))}
      </List>
    </>
  );
};

interface LuciBisectionHeuristicResultProps {
  heuristic_result: HeuristicAnalysis;
}

export const LuciBisectionHeuristicResult = ({
  heuristic_result,
}: LuciBisectionHeuristicResultProps) => {
  const suspects = heuristic_result?.suspects ?? [];
  if (suspects.length == 0) {
    return (
      <>LUCI Bisection couldn't find any heuristic suspect for this failure.</>
    );
  }
  const heuristicTooltipTitle =
    'The heuristic suspects were based on best-guess effort, so they may not be 100% accurate.';
  return (
    <>
      <TableContainer>
        <Table sx={{ maxWidth: '1000px', tableLayout: 'fixed' }}>
          <TableHead>
            <TableRow>
              <TableCell align="left" sx={{ width: '350px' }}>
                Heuristic result
                {/* We dont use Tooltip from MUI here as the MUI tooltip is attached in <body> tag
                  so style cannot be applied. */}
                <span title={heuristicTooltipTitle}>
                  <IconButton>
                    <HelpOutline></HelpOutline>
                  </IconButton>
                </span>
              </TableCell>
              <TableCell align="left" sx={{ width: '60px' }}>
                Confidence
              </TableCell>
              <TableCell align="left">Justification</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {suspects.map((s) => (
              <TableRow>
                <TableCell align="left">
                  <Link
                    href={s.reviewUrl}
                    target="_blank"
                    rel="noopener"
                    onClick={() => {
                      ga('send', {
                        hitType: 'event',
                        eventCategory: 'LuciBisection',
                        eventAction: 'ClickSuspectLink',
                        eventLabel: s.reviewUrl,
                        transport: 'beacon',
                      });
                    }}
                  >
                    {s.reviewUrl}
                  </Link>
                </TableCell>
                <TableCell align="left">
                  {confidenceText(s.confidence_level)}
                </TableCell>
                <TableCell align="left">
                  <pre style={{ whiteSpace: 'pre-wrap' }}>
                    {shortenJustification(s.justification)}
                  </pre>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </>
  );
};

interface LuciBisectionNthSectionResultProps {
  nth_section_result: NthSectionAnalysis | undefined;
}

export const LuciBisectionNthSectionResult = ({
  nth_section_result,
}: LuciBisectionNthSectionResultProps) => {
  if (nth_section_result === undefined) {
    return <></>;
  }
  if (
    !nth_section_result.suspect &&
    !nth_section_result.remaining_nth_section_range
  ) {
    return <>Nthsection could not find any suspects</>;
  }

  if (nth_section_result.suspect) {
    return (
      <>
        Nthsection suspect: &nbsp;
        <Link
          href={nth_section_result.suspect.reviewUrl}
          target="_blank"
          rel="noopener"
          onClick={() => {
            ga('send', {
              hitType: 'event',
              eventCategory: 'LuciBisection',
              eventAction: 'ClickSuspectLink',
              eventLabel: nth_section_result.suspect.reviewUrl,
              transport: 'beacon',
            });
          }}
        >
          {nth_section_result.suspect.reviewTitle ??
            nth_section_result.suspect.reviewUrl}
        </Link>
      </>
    );
  }

  if (nth_section_result.remaining_nth_section_range) {
    const rr = nth_section_result.remaining_nth_section_range;
    const rrLink = linkForRegressionRange(rr);
    return (
      <>
        Nthsection remaining regression range: &nbsp;
        <Link
          href={rrLink}
          target="_blank"
          rel="noopener"
          onClick={() => {
            ga('send', {
              hitType: 'event',
              eventCategory: 'LuciBisection',
              eventAction: 'ClickRegressionLink',
              eventLabel: rrLink,
              transport: 'beacon',
            });
          }}
        >
          {displayForRegressionRange(rr)}
        </Link>
      </>
    );
  }
  return <></>;
};

function linkForRegressionRange(rr: RegressionRange): string {
  return `https://${rr.last_passed.host}/${rr.last_passed.project}/+log/${rr.last_passed.id}..${rr.first_failed.id}`;
}

function displayForRegressionRange(rr: RegressionRange): string {
  return shortHash(rr.last_passed) + ':' + shortHash(rr.first_failed);
}

function shortHash(commit: GitilesCommit): string {
  return commit.id.substring(0, 7);
}

export const LuciBisectionResultSection = (
    props: LuciBisectionResultSectionProps,
) => {
  if (props.result == null) {
    return <></>;
  }

  if (!props.result.is_supported) {
    return <>LUCI Bisection does not support the failure in this builder.</>;
  }

  if (!props.result.analysis) {
    return <>LUCI Bisection couldn't find an analysis for this failure.</>;
  }

  // If there is a culprit, display it
  const culprits = props.result.analysis.culprits ?? [];
  if (culprits.length > 0) {
    return (
      <LuciBisectionCulpritSection
        culprits={culprits}
        failed_bbid={props.result.failed_bbid}
      ></LuciBisectionCulpritSection>
    );
  }

  return (
    <>
      <Link
        href={generateAnalysisUrl(props.result.failed_bbid)}
        target="_blank"
        rel="noopener"
      >
        LUCI Bisection
      </Link>
      &nbsp; results
      <Box sx={{ margin: '5px' }}>
        <LuciBisectionNthSectionResult
          nth_section_result={props.result.analysis.nth_section_result}
        ></LuciBisectionNthSectionResult>
      </Box>
      <Box sx={{ margin: '5px' }}>
        <LuciBisectionHeuristicResult
          heuristic_result={props.result.analysis.heuristic_result}
        ></LuciBisectionHeuristicResult>
      </Box>
    </>
  );
};

// Sometimes justification is too long if a CL touches many files.
// In such case we should shorten the justification to at most 3 lines
// and link to the detailed analysis if the sheriff wants to see the details
// (when the detail analysis page is ready).
function shortenJustification(justification: string) {
  const lines = justification.split('\n');
  if (lines.length < 4) {
    return justification;
  }
  return lines.slice(0, 3).join('\n') + '\n...';
}

function confidenceText(confidenceLevel: number) {
  switch (confidenceLevel) {
    case 1:
      return 'Low';
    case 2:
      return 'Medium';
    case 3:
      return 'High';
    default:
      return 'N/A';
  }
}

function getCulpritDisplayUrl(c: Culprit) {
  if (c.review_title == '' || c.review_title == null) {
    return c.review_url;
  }
  return c.review_title;
}

function generateAnalysisUrl(bbid: string) {
  return 'https://ci.chromium.org/ui/bisection/analysis/b/' + bbid;
}

export class SomLuciBisectionResult extends HTMLElement {
  cache: EmotionCache;
  child: HTMLDivElement;
  props: LuciBisectionResultSectionProps = {
    result: null,
  };

  constructor() {
    super();
    const root = this.attachShadow({ mode: 'open' });
    const parent = document.createElement('div');
    this.child = document.createElement('div');
    root.appendChild(parent).appendChild(this.child);
    this.cache = createCache({
      key: 'luci-bisection-result',
      container: parent,
    });
  }

  connectedCallback() {
    this.render();
  }

  set result(result: LuciBisectionResult) {
    this.props.result = result;
    this.render();
  }

  render() {
    if (!this.isConnected) {
      return;
    }
    render(
        <CacheProvider value={this.cache}>
          <LuciBisectionResultSection {...this.props} />
        </CacheProvider>,
        this.child,
    );
  }
}

customElements.define('som-luci-bisection-result', SomLuciBisectionResult);
