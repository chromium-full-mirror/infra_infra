# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from collections import namedtuple
from collections import defaultdict
import functools
import json
import importlib.resources as res

import os
import re

from analysis.component import Component
from analysis.occurrence import RankByOccurrence

def MergeComponents(components):
  """Given a list of components, merges components with the same hierarchy.

    For components with same hierarchy, return the most fine-grained component.
    For example, if components are ['Blink', 'Blink>Editing'], we should only
    return ['Blink>Editing'].
    """

  if not components or len(components) == 1:
    return components

  components.sort()
  merged_components = []
  index = 1
  while index < len(components):
    if not components[index].startswith(components[index - 1] + '>'):
      merged_components.append(components[index - 1])

    index += 1

  merged_components.append(components[-1])
  return merged_components


class ComponentClassifier(object):
  """Determines the component of a crash.

  For example: ['Blink>DOM', 'Blink>HTML'].
  """

  def __init__(self, components, top_n_frames, repo_to_dep_path,
               buganzier_component_dict):
    """Build a classifier for components.

    Args:
      components (list of crash.component.Component): the components to
        check for.
      top_n_frames (int): how many frames of the callstack to look at.
      buganzier_component_dict (dict): buganzier component and dir map
    """
    super(ComponentClassifier, self).__init__()
    self.components = components or []
    self.top_n_frames = top_n_frames
    self.repo_to_dep_path = repo_to_dep_path
    self.buganzier_component_dict = buganzier_component_dict

  def _RepoUrlToDepPath(self, repo_url):
    repo_url_without_git = (repo_url[:-len('.git')] if repo_url.endswith('.git')
                            else repo_url)
    repo_url_git = repo_url_without_git + '.git'
    return (self.repo_to_dep_path.get(repo_url_git) or
            self.repo_to_dep_path.get(repo_url_without_git, ''))

  def ClassifyFilePath(self, file_path):
    """Determines which component is responsible for this file_path."""
    component_to_dir_level = {}
    for component in self.components:
      match, directory = component.MatchesFilePath(file_path)
      if match:
        component_to_dir_level[component.component_name] = directory.count('/')

    # Returns components with longest directory path.
    return (max(component_to_dir_level,
                key=lambda component: component_to_dir_level[component])
            if component_to_dir_level else None)

  def ClassifyStackFrame(self, frame):
    """Determines which component is responsible for this frame."""
    if not frame.dep_path or not frame.file_path:
      return None

    dep_path = self._RepoUrlToDepPath(frame.repo_url) or frame.dep_path
    file_path = os.path.join(dep_path, frame.file_path)
    return self.ClassifyFilePath(file_path)

  def ClassifyRepoUrl(self, repo_url):
    """Determines which component is responsible for this repository."""
    dep_path = self._RepoUrlToDepPath(repo_url)
    component = self.ClassifyFilePath(dep_path + '/')
    return [component] if component else []

  def ClassifyTouchedFile(self, dep_path, touched_file):
    """Determine which component is responsible for a touched file."""
    file_path = os.path.join(dep_path, touched_file.changed_path)
    return self.ClassifyFilePath(file_path)

  # TODO(http://crbug.com/657177): return the Component objects
  # themselves, rather than strings naming them.
  def ClassifyCallStack(self, stack, top_n_components=2):
    """Classifies component of a crash.

    Args:
      stack (CallStack): The callstack that caused the crash.
      top_n_components (int): The number of top components for the stack,
        defaults to 2.

    Returns:
      List of top n components.
    """
    components = list(
        map(self.ClassifyStackFrame, stack.frames[:self.top_n_frames]))
    return MergeComponents(RankByOccurrence(components, top_n_components))

  def GetFilePathsFromCallStack(self, stack):
    """Return the file paths for the given call stack"""
    file_paths = []
    for frame in stack.frames[:self.top_n_frames]:
      if frame.dep_path and frame.file_path:
        dep_path = self._RepoUrlToDepPath(frame.repo_url) or frame.dep_path
        file_paths.append(os.path.join(dep_path, frame.file_path))
    return file_paths

  def GetBuganizerComponentIDFromSuspectedFilePaths(self, suspected_file_paths):
    """
    Return buganizer componentID with the highest occurrence for the given
    suspected file paths.
    """
    if not suspected_file_paths:
      return None
    counter = defaultdict(int)
    for file_path in suspected_file_paths:
      component_id = self._GetBuganizerComponentIDFromSuspectedFilePath(
          file_path.split('/')[1:])
      if component_id:
        counter[component_id] += 1
    return self._SortByOccurrence(counter)

  def _GetBuganizerComponentIDFromSuspectedFilePath(self, tokens):
    if not tokens:
      return None
    file_path = '/'.join(tokens)
    if file_path not in self.buganzier_component_dict:
      tokens.pop()
      return self._GetBuganizerComponentIDFromSuspectedFilePath(tokens)
    if 'buganizerPublic' not in self.buganzier_component_dict[file_path] or \
        'componentId' not in \
        self.buganzier_component_dict[file_path]['buganizerPublic']:
      return None
    return self.buganzier_component_dict[file_path]['buganizerPublic'][
        'componentId']

  def _SortByOccurrence(self, counter):
    key = None
    for k in counter:
      if not key:
        key = k
      elif counter[k] > counter[key]:
        key = k
    return key
