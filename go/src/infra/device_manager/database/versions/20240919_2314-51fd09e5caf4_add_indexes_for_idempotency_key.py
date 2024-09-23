# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""add indexes for idempotency_key

Revision ID: 51fd09e5caf4
Revises: 88e5695c7acd
Create Date: 2024-09-19 23:14:45.911904

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = '51fd09e5caf4'
down_revision: Union[str, None] = '88e5695c7acd'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  op.create_index("DeviceLeaseRecords_idempotency_key", "DeviceLeaseRecords",
                  ["idempotency_key"], unique=True)
  op.create_index("ExtendLeaseRequests_idempotency_key", "ExtendLeaseRequests",
                  ["idempotency_key"], unique=True)


def downgrade() -> None:
  op.drop_index("DeviceLeaseRecords_idempotency_key", "DeviceLeaseRecords")
  op.drop_index("ExtendLeaseRequests_idempotency_key", "ExtendLeaseRequests")
