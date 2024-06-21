// Copyright 2024 The Chromium Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import { Palette, Table, TableRow, useTheme } from '@mui/material';
import { blue, grey } from '@mui/material/colors';
import { ReactNode, forwardRef } from 'react';
import {
  ItemProps,
  TableComponents,
  TableVirtuoso,
  TableVirtuosoHandle,
} from 'react-virtuoso';

import { LogsTableEntry } from '@/types';

const zebraColor = (palette: Palette, index: number) => {
  // Zebra styling
  if (index % 2) return palette.mode === 'light' ? grey[200] : grey[700];
  return '';
};

const LogsTableBody = forwardRef<HTMLTableSectionElement>((props, ref) => (
  <tbody {...props} ref={ref} />
));
LogsTableBody.displayName = 'LogsTableBody';

interface Props {
  entries: LogsTableEntry[];
  onRowClick?: (
    index: number,
    event: React.MouseEvent<HTMLTableRowElement, MouseEvent>,
  ) => void;
  onRowMouseDown?: (
    index: number,
    event: React.MouseEvent<HTMLTableRowElement, MouseEvent>,
  ) => void;
  onRangeChanged?: () => void;
  getRowColor?: (entryId: string, index: number) => string;
  fixedHeaderContent?: () => ReactNode;
  fixedFooterContent?: () => ReactNode;
  rowContent: (index: number, row: LogsTableEntry) => ReactNode;
  initialTopMostItemIndex: number;
  disableVirtualization?: boolean;
}

export const VirtualizedTable = forwardRef<TableVirtuosoHandle | null, Props>(
  function VirtualizedTable(
    {
      entries,
      onRowClick,
      rowContent,
      initialTopMostItemIndex,
      disableVirtualization,
      fixedHeaderContent,
      fixedFooterContent,
      getRowColor,
      onRowMouseDown,
      onRangeChanged,
    }: Props,
    ref,
  ) {
    const { palette } = useTheme();

    const VirtuosoTableComponents: TableComponents<LogsTableEntry> = {
      Table: (props) => (
        <Table
          stickyHeader
          {...props}
          sx={{
            borderCollapse: 'separate',
            tableLayout: 'fixed',
            overflow: 'auto',
          }}
        />
      ),
      TableRow: ({
        item,
        'data-index': index,
        ...props
      }: ItemProps<LogsTableEntry>) => (
        <TableRow
          hover
          data-testid={`logs-table-row-${index}`}
          onMouseDown={(event) =>
            onRowMouseDown && onRowMouseDown(index, event)
          }
          onClick={(event) => onRowClick && onRowClick(index, event)}
          sx={{
            backgroundColor:
              getRowColor?.(item.entryId, index) ?? zebraColor(palette, index),
            '&:hover': {
              backgroundColor: `${
                palette.mode === 'light' ? blue[50] : blue[900]
              } !important`,
            },
          }}
          data-index={index}
          {...props}
        />
      ),
      TableBody: LogsTableBody,
    };

    return (
      <TableVirtuoso
        data-testid="logs-table"
        ref={ref}
        data={entries}
        totalCount={entries.length}
        components={VirtuosoTableComponents}
        fixedHeaderContent={fixedHeaderContent}
        fixedFooterContent={fixedFooterContent}
        itemContent={rowContent}
        rangeChanged={onRangeChanged}
        initialTopMostItemIndex={initialTopMostItemIndex}
        // Setting this to total count disables virtualization
        initialItemCount={disableVirtualization ? entries.length : undefined}
        key={disableVirtualization ? entries.length : undefined}
      />
    );
  },
);
