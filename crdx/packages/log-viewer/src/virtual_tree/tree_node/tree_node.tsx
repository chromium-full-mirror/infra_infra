// Copyright 2024 The Chromium Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
/* eslint-disable jsx-a11y/no-static-element-interactions */

import {
  ChevronRight as ChevronRightIcon,
  ExpandMore as ExpandMoreIcon,
} from '@mui/icons-material';
import { Stack, Typography } from '@mui/material';
import { deepOrange, lightBlue, yellow } from '@mui/material/colors';
import React, { useEffect, useState } from 'react';

import { TreeData, TreeNodeData } from '../types';

// Default active selection background color
export const ACTIVE_NODE_SELECTION_BACKGROUND_COLOR = deepOrange[300];

// Default matched search background color
export const SEARCH_MATCHED_BACKGROUND_COLOR = yellow[400];

// Default selected background color
export const SELECTED_NODE_BACKGROUND_COLOR = lightBlue[50];

/**
 * Props for default node renderer.
 */
export interface TreeNodeProps<T extends TreeNodeData> {
  data: TreeData<T>;
  collapseIcon?: React.ReactNode;
  expandIcon?: React.ReactNode;
  isSelected?: boolean;
  isSearchMatch?: boolean;
  isActiveSelection?: boolean;
  onNodeSelect?: (treeNodeData: TreeData<T>) => void;
  onNodeToggle?: (treeNodeData: TreeData<T>) => void;
}

/**
 * Returns default tree node component with basic features of
 * displaying the data and node toggle and node select props.
 */
export function TreeNode<T extends TreeNodeData>({
  data,
  collapseIcon,
  expandIcon,
  isSelected,
  isSearchMatch,
  isActiveSelection,
  onNodeSelect,
  onNodeToggle,
}: TreeNodeProps<T>) {
  const [treeNodeData, setTreeNodeData] = useState<TreeData<T>>(data);

  // Gets background to the node based on the treeData attribute.
  const getNodeBackgroundColor = () => {
    if (isActiveSelection) return ACTIVE_NODE_SELECTION_BACKGROUND_COLOR;
    if (isSearchMatch) return SEARCH_MATCHED_BACKGROUND_COLOR;
    if (isSelected && treeNodeData.isLeafNode)
      return SELECTED_NODE_BACKGROUND_COLOR;
    return undefined;
  };

  useEffect(() => {
    setTreeNodeData(data);
  }, [data]);

  return (
    <div
      data-testid={`default-tree-node-${data.id}`}
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        alignContent: 'center',
      }}
    >
      {treeNodeData.isLeafNode ? (
        <Typography
          component="span"
          sx={{ backgroundColor: getNodeBackgroundColor() }}
        >
          <div
            data-testid={`default-leaf-node-${data.id}`}
            style={{ cursor: 'pointer' }}
            onClick={() => onNodeSelect?.(treeNodeData)}
          >
            {treeNodeData.name}
          </div>
        </Typography>
      ) : (
        // indicates folder/directory to render the expand and collapse icons.
        <Stack
          spacing={0.5}
          direction={'row'}
          style={{ display: 'flex', alignItems: 'center' }}
        >
          <div
            data-testid={`default-node-${data.id}`}
            onClick={() => onNodeToggle?.(treeNodeData)}
            style={{ display: 'flex', alignItems: 'center' }}
          >
            {treeNodeData.isOpen
              ? collapseIcon ?? <ExpandMoreIcon sx={{ fontSize: '18px' }} />
              : expandIcon ?? <ChevronRightIcon sx={{ fontSize: '18px' }} />}
          </div>
          <Typography
            component="span"
            fontWeight="bold"
            data-testid={`name-${treeNodeData.data.name}`}
            sx={{
              display: 'flex',
              backgroundColor: getNodeBackgroundColor(),
            }}
          >
            <div
              style={{ cursor: 'pointer' }}
              onClick={() => onNodeSelect?.(treeNodeData)}
            >
              {treeNodeData.data.name}
            </div>
          </Typography>
        </Stack>
      )}
    </div>
  );
}
