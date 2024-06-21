// Copyright 2024 The Chromium Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import { TableRow, Box } from '@mui/material';
import { Meta, StoryObj } from '@storybook/react';

import { SortOrder } from '@/constants/table_constants';
import { LogsTableEntry } from '@/types';

import { LogsEntryTableCell } from './logs_cell';
import { LogsHeaderCell } from './logs_header_cell';
import { createMockLogTableEntriesForScreenRecorder } from './test_utils';
import { VirtualizedTable } from './virtualized_table';

const meta = {
  title: 'VirtualizedTable',
  component: VirtualizedTable,
  decorators: [
    (Story) => (
      <Box
        sx={{
          height: '300px',
          width: '100%',
          borderBottom: '1px solid #e0e0e0',
        }}
      >
        <Story />
      </Box>
    ),
  ],
  tags: ['autodocs'],
} satisfies Meta<typeof VirtualizedTable>;
export default meta;

type Story = StoryObj<typeof VirtualizedTable>;

const defaultArgs: Story['args'] = {
  entries: createMockLogTableEntriesForScreenRecorder('mock-path'),
  initialTopMostItemIndex: 0,
  fixedHeaderContent: () => {
    return (
      <TableRow>
        <LogsHeaderCell
          label="#"
          sortId="logFile"
          width="1px"
          sortOrder={SortOrder.DESC}
          onHeaderSort={() => {}}
        />
        <LogsHeaderCell
          label="ID"
          width="1rem"
          sortId="logFile"
          sortOrder={SortOrder.ASC}
          onHeaderSort={() => {}}
        />
        <LogsHeaderCell
          label="SUMMARY"
          sortId="logFile"
          sortOrder={SortOrder.ASC}
          onHeaderSort={() => {}}
          sortable
        />
      </TableRow>
    );
  },
  rowContent: (_: number, row: LogsTableEntry) => {
    return (
      <>
        <LogsEntryTableCell>{row.line! + 1}</LogsEntryTableCell>
        <LogsEntryTableCell>{row.severity}</LogsEntryTableCell>
        <LogsEntryTableCell>{row.summary}</LogsEntryTableCell>
      </>
    );
  },
};

export const Base: Story = {
  args: defaultArgs,
};

export const WithDifferentColors: Story = {
  args: {
    ...defaultArgs,
    getRowColor: (_, index) => (index % 2 === 0 ? 'grey' : 'lightblue'),
  },
};
