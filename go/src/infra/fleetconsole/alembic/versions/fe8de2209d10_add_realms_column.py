# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""add realms column

Revision ID: fe8de2209d10
Revises: 5d7c7d5e9b8f
Create Date: 2025-02-27 11:29:58.330922+00:00

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'fe8de2209d10'
down_revision: Union[str, None] = '5d7c7d5e9b8f'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
  op.add_column('Devices', sa.Column('realm', sa.String))
  pass


def downgrade() -> None:
  op.drop_column('Devices', 'realm')
