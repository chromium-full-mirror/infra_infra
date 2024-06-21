// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import { SummaryStyle } from '@/constants/table_constants';
import { LogsTableEntry } from '@/types';

export const createMockLogTableEntriesForScreenRecorder = (
  mockLogsPath: string,
): LogsTableEntry[] => {
  return Array(20)
    .fill(0)
    .map((_, idx) => {
      return {
        enableContextActionMenu: true,
        entryId: `${mockLogsPath}/dir1/log1:${idx}`,
        fileNumber: 1,
        fullName: `${mockLogsPath}/dir1/log${idx}`,
        line: idx,
        logFile: 'dir1/log1',
        normalizedSummary: '',
        summary: `Test info ... ${idx}`,
        timestamp: `2023-10-01T11:22:33.${String(idx).padStart(3, '0')}Z`,
        severity: 'INFO',
        summaryStyle: SummaryStyle.DEFAULT,
      };
    });
};
