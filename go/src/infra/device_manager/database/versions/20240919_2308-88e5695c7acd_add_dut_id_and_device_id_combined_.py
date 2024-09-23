# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""add dut_id and Device id combined indexes

Revision ID: 88e5695c7acd
Revises: e0729dfd68dc
Create Date: 2024-09-19 23:08:34.078258

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = '88e5695c7acd'
down_revision: Union[str, None] = 'e0729dfd68dc'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  op.create_index("Devices_id_dutid", "Devices", ["id", "dut_id"])
  op.create_index("DeviceLeaseRecords_deviceid_dutid", "DeviceLeaseRecords",
                  ["device_id", "dut_id"])


def downgrade() -> None:
  op.drop_index("Devices_id_dutid", "Devices")
  op.drop_index("DeviceLeaseRecords_deviceid_dutid", "DeviceLeaseRecords")
