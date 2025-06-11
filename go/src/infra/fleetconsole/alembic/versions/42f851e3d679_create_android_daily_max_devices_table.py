# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""create_android_daily_max_devices_table

Revision ID: 42f851e3d679
Revises: 96e47545eef0
Create Date: 2025-05-14 08:34:41.862208+00:00

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = '42f851e3d679'
down_revision: Union[str, None] = '96e47545eef0'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  _ = op.create_table(
      'android_daily_max_devices',
      sa.Column(
          'id',
          sa.Integer,
          sa.Identity(start=1),
          primary_key=True,
          nullable=False),
      sa.Column('lab_name', sa.String, nullable=True),
      sa.Column('host_group', sa.String, nullable=True),
      sa.Column('run_target', sa.String, nullable=True),

      # The maximum number of devices allocated that day
      sa.Column('max_devices_allocated', sa.Integer, nullable=False),

      # When the record is first created truncated to the start of the day
      sa.Column(
          'period_start',
          sa.DateTime,
          server_default=sa.text(
              "date_trunc('day', (now() AT TIME ZONE 'utc'))"),
          nullable=False))


def downgrade() -> None:
  op.drop_table('android_daily_max_devices')
