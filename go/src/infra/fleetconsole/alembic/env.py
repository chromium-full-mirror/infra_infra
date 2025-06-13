# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
import shutil
import subprocess

from configparser import Error
from logging.config import fileConfig

from sqlalchemy import engine_from_config
from sqlalchemy import pool

from alembic import context

# this is the Alembic Config object, which provides
# access to the values within the .ini file in use.
config = context.config

# Interpret the config file for Python logging.
# This line sets up loggers basically.
if config.config_file_name is not None:
  fileConfig(config.config_file_name)


# Allow user to overwrite the port number from the command line, e.g.
# ALEMBIC_ENV=foo alembic -x port=<port> upgrade head
env = context.get_x_argument(as_dictionary=True).get("env")
if env is None:
  env = "local"

if env not in ("local", "dev", "prod"):
  raise Error('env must be either "local", "dev" or "prod" not ' + env)
if env == 'local':
  config.set_main_option(
      "sqlalchemy.url",
      "postgresql://postgres:password@localhost:5432/postgres")
else:
  # Expecting there to be an ssh tunnel to the db.
  # See README.md on how to do that
  gcloud_path = shutil.which("gcloud")
  if gcloud_path is None:
    raise FileNotFoundError("gcloud command not found")

  command: list[str] = [
      gcloud_path, "secrets", "versions", "access", "latest",
      f'--project=fleet-console-{env}', "--secret=db-password"
  ]

  db_password = subprocess.run(command, capture_output=True, text=True)
  if db_password.returncode != 0:
    raise RuntimeError(f"Failed to get db password: {db_password.stderr}")

  db_uri = f"postgresql://postgres:{db_password.stdout.strip()}@localhost:5432/console_db"
  config.set_main_option("sqlalchemy.url", db_uri)


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
