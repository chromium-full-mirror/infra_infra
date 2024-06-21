// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import { SummaryStyle } from '@/constants';

export interface LogsTableEntry {
  entryId: string;
  logFile?: string;
  fileNumber?: number;
  timestamp?: string;
  severity?: string;
  summary: string;
  fullName?: string;
  line?: number;
  summaryStyle?: SummaryStyle;
  normalizedSummary?: string;
  // Whether to enable the context action menu.
  enableContextActionMenu?: boolean;
  // Comparison mode controls
  chunkId?: number;
  enableExpansion?: boolean;
}
