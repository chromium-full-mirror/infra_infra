FROM python:3.9-slim

WORKDIR /app

COPY requirements.in .
RUN pip install --no-cache-dir pip-tools==7.4.1 && \
  pip-compile --generate-hashes requirements.in && \
  pip install --no-cache-dir -r requirements.txt

COPY alembic.ini .
COPY database database

CMD ["alembic", "upgrade", "head"]
