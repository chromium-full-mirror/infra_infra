# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""create devices table

Revision ID: 5d7c7d5e9b8f
Revises:
Create Date: 2025-01-31 14:48:07.990732+00:00

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects.postgresql import JSONB

# revision identifiers, used by Alembic.
revision: str = '5d7c7d5e9b8f'
down_revision: Union[str, None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  op.create_table(
      'Devices',
      sa.Column('id', sa.String, primary_key=True),
      sa.Column('dut_id', sa.String, nullable=False),
      sa.Column('host', sa.String, nullable=False),
      sa.Column('port', sa.String, nullable=False),
      sa.Column('type', sa.String, nullable=False),
      sa.Column('state', sa.String, nullable=False),
      sa.Column('labels', JSONB),
  )


def downgrade() -> None:
  op.drop_table('Devices')
