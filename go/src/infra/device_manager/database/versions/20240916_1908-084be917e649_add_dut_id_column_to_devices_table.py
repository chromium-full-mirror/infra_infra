# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""add dut_id column to Devices table

Revision ID: 084be917e649
Revises: b56ba79423b3
Create Date: 2024-09-16 19:08:09.742713

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = '084be917e649'
down_revision: Union[str, None] = 'b56ba79423b3'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  # Column for dut_id (asset tag) with unique index
  op.add_column("Devices", sa.Column("dut_id", sa.String))
  op.create_index("Devices_dut_id", "Devices", ["dut_id"])


def downgrade() -> None:
  op.drop_index("Devices_dut_id", "Devices")
  op.drop_column("Devices", "dut_id")
