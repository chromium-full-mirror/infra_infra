# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import configparser
from logging.config import fileConfig
import urllib.parse
import os

from sqlalchemy import engine_from_config
from sqlalchemy import pool

from alembic import context

# this is the Alembic Config object, which provides
# access to the values within the .ini file in use.
config = context.config
alembic_ini = config.config_ini_section

# interpolate vars to alembic.ini from config file
db_config = configparser.ConfigParser()
db_config.read('./database/db_config.ini')
db_env = os.environ['ALEMBIC_ENV']

config.set_section_option(alembic_ini, 'DB_USER', db_config[db_env]['DB_USER'])
config.set_section_option(alembic_ini, 'DB_PASS', db_config[db_env]['DB_PASS'])
config.set_section_option(alembic_ini, 'DB_HOST', db_config[db_env]['DB_HOST'])
config.set_section_option(alembic_ini, 'DB_PORT', db_config[db_env]['DB_PORT'])
config.set_section_option(alembic_ini, 'DB_NAME', db_config[db_env]['DB_NAME'])

# Allow user to overwrite the port number from the command line, e.g.
# ALEMBIC_ENV=foo alembic -x port=<port> upgrade head
port = context.get_x_argument(as_dictionary=True).get("port")
if port:
  config.set_section_option(alembic_ini, "DB_PORT", port)

# Interpret the config file for Python logging.
# This line sets up loggers basically.
if config.config_file_name is not None:
  fileConfig(config.config_file_name)

# add your model's MetaData object here
# for 'autogenerate' support
# from myapp import mymodel
# target_metadata = mymodel.Base.metadata
target_metadata = None

# other values from the config, defined by the needs of env.py,
# can be acquired:
# my_important_option = config.get_main_option("my_important_option")
# ... etc.


def run_migrations_offline() -> None:
  """Run migrations in 'offline' mode.

    This configures the context with just a URL
    and not an Engine, though an Engine is acceptable
    here as well.  By skipping the Engine creation
    we don't even need a DBAPI to be available.

    Calls to context.execute() here emit the given string to the
    script output.

    """
  url = config.get_main_option("sqlalchemy.url")
  context.configure(
      url=url,
      target_metadata=target_metadata,
      literal_binds=True,
      dialect_opts={"paramstyle": "named"},
  )

  with context.begin_transaction():
    context.run_migrations()


def run_migrations_online() -> None:
  """Run migrations in 'online' mode.

    In this scenario we need to create an Engine
    and associate a connection with the context.

    """
  connectable = engine_from_config(
      config.get_section(config.config_ini_section, {}),
      prefix="sqlalchemy.",
      poolclass=pool.NullPool,
  )

  with connectable.connect() as connection:
    context.configure(connection=connection, target_metadata=target_metadata)

    with context.begin_transaction():
      context.run_migrations()


if context.is_offline_mode():
  run_migrations_offline()
else:
  run_migrations_online()
