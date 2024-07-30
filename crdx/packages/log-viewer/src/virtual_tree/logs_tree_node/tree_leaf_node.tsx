// Copyright 2024 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import {
  InsertDriveFileOutlined as InsertDriveFileOutlinedIcon,
  InfoOutlined as InfoOutlinedIcon,
} from '@mui/icons-material';
import { Stack, Typography } from '@mui/material';
import { grey } from '@mui/material/colors';
import prettyBytes from 'pretty-bytes';
import { ReactNode } from 'react';

import {
  TreeData,
  TreeFontVariant,
  TreeNodeColors,
  TreeNodeLabels,
  ObjectNode,
} from '../types';
import { getNodeBackgroundColor } from '../utils';

import { LeafNodeText } from './leaf_node_text';

// Total width of 3 inline action icons of 24px each.
const INLINE_ACTIONS_WIDTH = '72px';

/**
 * Props for the tree lead node.
 */
interface TreeLeafNodeProps {
  treeNodeData: TreeData<ObjectNode>;
  treeFontSize?: TreeFontVariant;
  iconFontSize?: string;
  isSearchMatch?: boolean;
  isActiveSelection?: boolean;
  isSpecialNode?: boolean;
  isHovered?: boolean;
  inlineActions?: ReactNode;
  labels: TreeNodeLabels;
  colors?: TreeNodeColors;
  onLeafNodeClick?: (
    newlySelectedNode: ObjectNode,
    mergeFile?: boolean,
  ) => void;
  onMouseEnter: () => void;
  onMouseLeave: () => void;
}

export function TreeLeafNode({
  treeNodeData,
  treeFontSize,
  iconFontSize,
  isSearchMatch,
  isActiveSelection,
  isSpecialNode,
  isHovered,
  inlineActions,
  labels,
  colors,
  onLeafNodeClick,
  onMouseEnter,
  onMouseLeave,
}: TreeLeafNodeProps) {
  const nodeDataTestId = onLeafNodeClick
    ? `name-${treeNodeData.name}-with-leaf-handler`
    : `name-${treeNodeData.name}`;
  // Checks if the file is supported.
  const textColorStyle = !treeNodeData.data.viewingsupported
    ? { color: colors?.unsupportedColor ?? grey[600] }
    : {};
  return (
    <Stack direction="row" spacing={1}>
      <Typography
        variant={treeFontSize ?? /* default value */ 'body1'}
        component="span"
        sx={{
          display: 'flex',
          flexWrap: 'nowrap',
          flexDirection: 'row',
          ...textColorStyle,
        }}
      >
        <InsertDriveFileOutlinedIcon
          sx={{ fontSize: iconFontSize ?? '18px' }}
        />
        <span>
          <span
            data-testid={nodeDataTestId}
            css={{
              backgroundColor: getNodeBackgroundColor(
                colors,
                treeNodeData.data.deeplinked,
                isActiveSelection,
                isSearchMatch,
              ),
            }}
          >
            <LeafNodeText
              node={treeNodeData.data}
              hasLeafNodeClick={!!onLeafNodeClick}
              colors={colors}
              labels={labels}
            />
          </span>
          <span data-testid={`size-${treeNodeData.name}`}>
            {` [${prettyBytes(treeNodeData.data.size || 0)}]`}
          </span>
        </span>
        {isSpecialNode && (
          <InfoOutlinedIcon
            color="warning"
            sx={{ fontSize: iconFontSize ?? '18px', ml: 0.5 }}
            titleAccess={labels.specialNodeInfoTooltip}
          />
        )}
        {/* Adding this span with fixed width accounts for the width of the
       inline actions. This prevents rearranging of the long format texts
       on hover. */}
        <span
          css={{
            minWidth: INLINE_ACTIONS_WIDTH,
            maxWidth: INLINE_ACTIONS_WIDTH,
            display: 'flex',
            flexDirection: 'row',
            marginLeft: '5px',
          }}
          onMouseEnter={onMouseEnter}
          onMouseLeave={onMouseLeave}
        >
          {isHovered && inlineActions}
        </span>
      </Typography>
    </Stack>
  );
}
