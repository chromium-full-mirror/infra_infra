# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""create_android_hosts_table

Revision ID: 96e47545eef0
Revises: e5c4b15c0213
Create Date: 2025-05-14 08:33:24.746105+00:00

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa

# revision identifiers, used by Alembic.
revision: str = '96e47545eef0'
down_revision: Union[str, None] = 'e5c4b15c0213'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  _ = op.create_table(
      'android_hosts',
      sa.Column('hostname', sa.String, primary_key=True),
      sa.Column('host_group', sa.String, nullable=True),
      sa.Column('state', sa.String, nullable=True),
  )


def downgrade() -> None:
  op.drop_table('android_hosts')
