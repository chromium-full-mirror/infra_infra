# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import typing

from protos.ttcp.solver import solver_pb2


class SolvedResults:

  def __init__(self, solved_results: solver_pb2.SolvedCategory, pool: str):
    self._solved_results = solved_results
    self._pool = pool

  @property
  def pool(self) -> str:
    return self._pool

  def get_eqc_legacy_solutions(
      self,) -> typing.Dict[str, typing.List[solver_pb2.LegacyTarget]]:
    """Generates a list of legacy targets from solved results.

        Returns:
          List of legacy targets.
        """
    classes = [component for component in self._solved_results.classes]
    eqc_map = {
        (cls.name if cls.name else f"class-{idx}"): cls.legacy_solutions
        for idx, cls in enumerate(classes)
    }
    return eqc_map

  def get_models_by_eqcs(self) -> typing.Dict[str, typing.List[str]]:
    """Gets a mapping of models to eqc_names from a solved result.

        Returns:
          Dictionary where key=<eqc_name> and value=<list of models>.
        """
    eqc_legacy_solutions = self.get_eqc_legacy_solutions()
    eqc_models = {}
    for eqc_name, legacy_sol_list in sorted(eqc_legacy_solutions.items()):
      model_lists = [sol.models for sol in legacy_sol_list]
      models = sorted(
          [model for model_list in model_lists for model in model_list])
      eqc_models.update({eqc_name: models})
    return eqc_models

  def get_eqc_targets(self) -> typing.Dict[str, solver_pb2.SolvedTarget]:
    """Generates a dictionary of eqc names and  targets from solved results.

        Returns:
          Dictionary of eqc names and associated targets.
        """
    classes = [component for component in self._solved_results.classes]
    eqc_map = {
        (cls.name if cls.name else f"class-{idx}"): cls.targets
        for idx, cls in enumerate(classes)
    }
    return eqc_map

  def get_hwids_by_eqcs(self) -> typing.Dict[str, typing.List[str]]:
    """Gets a mapping of hwids to eqc_names from a solved result.

        Returns:
          Dictionary where key=<eqc_name> and value=<list of hwids>.
        """
    eqc_hwids = {}
    eqc_targets = self.get_eqc_targets()

    for eqc_name, target_list in sorted(eqc_targets.items()):
      hwids = sorted({target.device_id for target in target_list.values()})
      eqc_hwids.update({eqc_name: hwids})
    return eqc_hwids

  def print_eqc_targets(self) -> None:
    """Prints the contents of a solved results by eqc and target groupings."""
    eqc_header = ("*" * 5) + "{}" + ("*" * 5)
    targets_header = "TARGETS\nCount: {}"
    eqc_targets = self.get_hwids_by_eqcs()
    for idx, (eqc_name, target_list) in enumerate(eqc_targets.items()):
      eqc_title = f"EQC-{idx + 1}: {eqc_name}"
      print(eqc_header.format(eqc_title))
      print(targets_header.format(len(target_list)))
      for target in target_list:
        print(target)
      print("\n")
