# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import collections

Key = collections.namedtuple('Key', ('kind', 'id'))


class Entity(dict):

  def __init__(self, key):
    self._key = key

  @property
  def key(self):
    return self._key

  @property
  def id(self):
    return self._key.id


class Query:

  def __init__(self, client: 'Client', kind: str = None):
    self._client = client
    self._kind = kind

  def fetch(self) -> list[Entity]:
    if not self._kind:
      return list(self._client.store.values())
    return [
        entity for entity in self._client.store.values()
        if entity.key.kind == self._kind
    ]


class Client:

  def __init__(self):
    self._store = {}

  def key(self, kind: str, key_id: str) -> Key:
    return Key(kind, key_id)

  def get(self, key: Key) -> Entity:
    return self._store.get(key)

  def put(self, entity: Entity):
    self._store[entity.key] = entity

  def query(self, **kwargs) -> Query:
    return Query(self, **kwargs)

  @property
  def store(self) -> dict[str, str]:
    return self._store


def MakeClient(values: dict[Key, dict] = None):

  def wrap():
    client = Client()
    if values:
      for key, value in values.items():
        entity = Entity(key)
        entity.update(value)
        client.store[key] = entity
    return client

  return wrap
