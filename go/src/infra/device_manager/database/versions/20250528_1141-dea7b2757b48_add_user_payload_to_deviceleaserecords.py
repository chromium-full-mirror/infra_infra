# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Add user_payload to DeviceLeaseRecords

Revision ID: dea7b2757b48
Revises: 51fd09e5caf4
Create Date: 2025-05-28 11:41:16.738444

"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

# revision identifiers, used by Alembic.
revision: str = "dea7b2757b48"
down_revision: Union[str, None] = "51fd09e5caf4"
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  op.add_column(
      "DeviceLeaseRecords",
      sa.Column("user_payload", postgresql.JSONB(), nullable=True),
  )


def downgrade() -> None:
  op.drop_column("DeviceLeaseRecords", "user_payload")
