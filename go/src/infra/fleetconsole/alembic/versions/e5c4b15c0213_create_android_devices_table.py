# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""create_android_devices_table

Revision ID: e5c4b15c0213
Revises: fe8de2209d10
Create Date: 2025-05-14 08:33:18.118105+00:00

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = 'e5c4b15c0213'
down_revision: Union[str, None] = 'fe8de2209d10'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  _ = op.create_table(
      'android_devices',
      sa.Column('id', sa.String, primary_key=True),
      sa.Column('lab_name', sa.String, nullable=True),
      sa.Column('host_group', sa.String, nullable=True),
      sa.Column('run_target', sa.String, nullable=True),
      sa.Column('state', sa.String, nullable=True),
  )


def downgrade() -> None:
  op.drop_table('android_devices')
