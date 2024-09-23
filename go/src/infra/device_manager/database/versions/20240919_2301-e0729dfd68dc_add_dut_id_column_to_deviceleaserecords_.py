# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""add dut_id column to DeviceLeaseRecords table

Revision ID: e0729dfd68dc
Revises: 084be917e649
Create Date: 2024-09-19 23:01:09.893456

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = 'e0729dfd68dc'
down_revision: Union[str, None] = '084be917e649'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  # Column for dut_id (asset tag) with unique index
  op.add_column("DeviceLeaseRecords", sa.Column("dut_id", sa.String))
  op.create_index("DeviceLeaseRecords_dut_id", "DeviceLeaseRecords", ["dut_id"])


def downgrade() -> None:
  op.drop_index("DeviceLeaseRecords_dut_id", "DeviceLeaseRecords")
  op.drop_column("DeviceLeaseRecords", "dut_id")
