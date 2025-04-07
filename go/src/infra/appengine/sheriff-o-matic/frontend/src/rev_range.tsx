// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import React, { useMemo } from 'react';
import { Box, Typography, Link } from '@mui/material';
import { RegressionRangeJson } from './server_json';

const parseCommitPosition = (pos: string): number | null => {
    const groups = /.*@{#([0-9]+)}/.exec(pos);
    if (groups && groups.length === 2) {
        return Number(groups[1]);
    }
    return null;
};

const shouldUseRangeRevisions = (range: RegressionRangeJson): boolean => {
    return !!(range.host && range.host.length > 0 &&
        range.revisions && range.revisions.length > 0 &&
        !range.positions); // Prioritize revisions if host exists and positions don't
};

const calculateRangeDisplay = (range: RegressionRangeJson): string => {
    if (!range) return '';

    let startStr = '';
    let endStr = '';

    if (shouldUseRangeRevisions(range) && range.revisions) {
        // Use first 7 chars for display (consistent with LuciBisectionResultSection helpers)
        startStr = range.revisions[0]?.substring(0, 7) ?? '';
        endStr = range.revisions[range.revisions.length - 1]?.substring(0, 7) ?? '';
    } else if (range.positions && range.positions.length > 0) {
        // Sort positions numerically before getting start/end
        const sortedPositions = range.positions
            .map(parseCommitPosition)
            .filter((num): num is number => num !== null)
            .sort((a, b) => a - b);

        if (sortedPositions.length > 0) {
            startStr = String(sortedPositions[0]);
            endStr = String(sortedPositions[sortedPositions.length - 1]);
        }
    }

    if (startStr && endStr && startStr !== endStr) {
        return `${startStr} - ${endStr}`;
    }
    return startStr; // Return only start if end is same or invalid
};

const calculateRangeLink = (range: RegressionRangeJson): string => {
    if (!range) return '';

    if (shouldUseRangeRevisions(range) && range.revisions && range.host) {
        const start = range.revisions[0];
        const end = range.revisions[range.revisions.length - 1];
        let host = range.host;
        if (host.startsWith('https://')) {
            host = host.substring('https://'.length);
        }
        return `https://${host}/${range.repo}/+log/${start}^..${end}`;
    } else if (range.positions && range.positions.length > 0) {
        const sortedPositions = range.positions
            .map(parseCommitPosition)
            .filter((num): num is number => num !== null)
            .sort((a, b) => a - b);

        if (sortedPositions.length > 0) {
            const startNum = sortedPositions[0];
            const endNum = sortedPositions[sortedPositions.length - 1];
            // Assume crrev.com link for position-based ranges
            return `https://crrev.com/${startNum}..${endNum}`;
        }
    }
    return '';
};


interface RevRangeProps {
    range: RegressionRangeJson | null | undefined;
}

export const RevRange = ({ range }: RevRangeProps) => {
    const hasError = !!range?.error;
    const displayRepo = range?.repo ?? '';

    const displayRange = useMemo(() => (range ? calculateRangeDisplay(range) : ''), [range]);
    const rangeLink = useMemo(() => (range ? calculateRangeLink(range) : ''), [range]);

    if (!range || hasError) {
        // Decide how to display errors or missing range
        if (hasError) {
            console.error("Regression range error:", range.error);
            return <Typography color="error">Error loading regression range.</Typography>;
        }
        return null;
    }

    return (
        <Box sx={{ mb: 1 }}>
            <Typography component="span" variant="body1" sx={{ fontWeight: 'bold', mr: 1 }}>
                Regression range:
            </Typography>
            {displayRepo && (
                <Typography component="span" sx={{ mr: 0.5 }}>
                    {displayRepo}:
                </Typography>
            )}
            {rangeLink && displayRange ? (
                <Link
                    href={rangeLink}
                    target="_blank"
                    rel="noopener noreferrer"
                    sx={{ mr: 0.5 }}
                >
                    {displayRange}
                </Link>
            ) : displayRange ? (
                <Typography component="span" sx={{ mr: 0.5 }}>
                    {displayRange}
                </Typography>
            ) : null}
        </Box>
    );
};