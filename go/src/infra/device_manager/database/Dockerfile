FROM python:3.9-slim

WORKDIR /app

COPY requirements.in .

# TODO(gregorynisbet): Re-enable require hashes once we figure out what the heck is going on.
# RUN pip install --no-cache-dir --require-hashes pip-tools==7.4.1 && \
#   pip-compile --generate-hashes requirements.in && \
#   pip install --no-cache-dir --require-hashes -r requirements.txt

RUN pip install --no-cache-dir pip-tools==7.4.1 && \
    pip install --no-cache-dir -r requirements.in

COPY alembic.ini .
COPY database database

CMD ["alembic", "upgrade", "head"]
