# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""create_android_repair_metrics_table

Revision ID: e024d8b511cf
Revises: 42f851e3d679
Create Date: 2025-05-14 08:35:03.590869+00:00

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = 'e024d8b511cf'
down_revision: Union[str, None] = '42f851e3d679'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  _ = op.create_table(
      'android_repair_metrics', sa.Column('priority', sa.String, nullable=True),
      sa.Column('lab_name', sa.String, nullable=True),
      sa.Column('host_group', sa.String, nullable=True),
      sa.Column('run_target', sa.String, nullable=True),
      sa.Column('minimum_repairs', sa.Integer, nullable=False),
      sa.Column('devices_offline', sa.Integer, nullable=False),
      sa.Column('total_devices', sa.Integer, nullable=False),
      sa.PrimaryKeyConstraint("lab_name", "host_group", "run_target"))


def downgrade() -> None:
  op.drop_table('android_repair_metrics')
