# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

#! /usr/bin/env nix-shell

import datetime
from datetime import datetime
import json
import os
import sys

sys.path.append(
    os.path.dirname(
        os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
from google.protobuf import json_format
from protos.ttcp.solver import solver_pb2
from protos.ttcp.syntax import syntax_pb2

from .advisor import Descriptor
from .advisor import ListLabInventory
from .advisor import Properties
from .command import Command
from .solve_expression import SolveExpression
from .solved_results import SolvedResults


class DDDCli:

  def __init__(self, ddd_dir: str = None):
    """Constructor for the ddd_cli object

        Args:
            ddd_dir (str): file path location of the ddd ddd_cli executable
        """
    ddd_cli_dir = os.path.dirname(os.path.realpath(__file__))
    if ddd_dir is None:
      ddd_dir = os.path.dirname(
          os.path.dirname(os.path.dirname(os.path.realpath(__file__))))
    self._ddd_dir = ddd_dir
    self._ddd_cli_dir = ddd_cli_dir
    self._init_data_dir()

  def _init_data_dir(self):
    """Private method to set class directory vars and generate directory paths."""
    self._data_dir = os.path.join(self._ddd_cli_dir, "data")
    self._variants_data_dir = os.path.join(self._data_dir, "variants")
    self._advisor_output_dir = os.path.join(self._data_dir, "advisor")
    for dir in (
        self._data_dir,
        self._variants_data_dir,
        self._advisor_output_dir,
    ):
      if not os.path.exists(dir):
        os.makedirs(dir)

  def execute_command(self,
                      command: Command,
                      output_file_path: str = None) -> dict:
    """Executes a ddd ddd_cli command

        Args:
            command (Command): ddd Command
            output_file_path (str, optional): File path to command output
            Defaults to None.

        Raises:
            Exception: Exception is thrown if command cannot complete execution

        Returns:
            dict: Data representation of the json command output
        """
    working_directory = os.getcwd()
    os.chdir(path=self._ddd_dir)
    return_code, stdout, stderr = command.execute()
    os.chdir(working_directory)

    if return_code != 0:
      raise Exception(stderr, stdout)
    data = {}
    if output_file_path:
      print(f"Successfully generated output file: {output_file_path}")
      with open(output_file_path, "r") as file:
        data = json.load(fp=file)
    return data

  def execute_solve_expression(
      self,
      variant_expression: syntax_pb2.CategoryExpression = None,
      variants_file: str = None,
      inventory_file_path: str = None,
      inventory_swarming_pool: str = None,
      class_filter: str = None,
      variant_expression_map=None,
  ) -> SolvedResults:
    """Executes the 'solve_expression' ddd ddd_cli command

        Args:
            variant_expression_map: dictionary representing the key value pair of
             the variant expression to solve
            variants_file:
            inventory_file_path:
            inventory_swarming_pool:
            class_filter:

        Returns:
            Data representation of the json command output
        """
    if variant_expression_map is None:
      variant_expression_map = json_format.MessageToDict(
          message=variant_expression)
    filename_prefix = ((variant_expression_map["name"].replace(":", "_")
                        if "name" in variant_expression_map else "default")
                       if variant_expression_map else "test_file_out")
    variants_filename = f"{filename_prefix}_expression_variants.json"
    encoded_variant_expression = (
        json.dumps(json.dumps(variant_expression_map))
        if variant_expression_map else None)
    output_file_path = os.path.join(self._variants_data_dir, variants_filename)

    command = SolveExpression(
        variants=encoded_variant_expression,
        variants_file=variants_file,
        outpath=output_file_path,
        inventory_swarming=(not bool(inventory_file_path)),
        inventory_swarming_pool=inventory_swarming_pool,
        inventory_file=inventory_file_path,
        class_filter=class_filter,
    )
    start = datetime.now()
    print(f"Solver Start: {start}")
    result = self.execute_command(
        command=command, output_file_path=output_file_path)
    end = datetime.now()
    duration = end - start
    print(f"Solver Finished: {end}")
    print(f"Duration: {duration}")
    solved_results = json_format.ParseDict(
        js_dict=result, message=solver_pb2.SolvedCategory())
    return SolvedResults(
        solved_results=solved_results, pool=inventory_swarming_pool)

  def execute_list_lab_inventory(self, outpath: str = None) -> dict:
    """Executes the listLabInventory advisor sub command

        Args:
            outpath (str, optional): output file path for the json file

        Returns:
            dict: Data representation of the json command output
        """
    if outpath is None:
      file_name = (f"lab_inventory_{datetime.now().strftime('%Y-%m-%d')}.json")
      outpath = os.path.join(self._advisor_output_dir, file_name)
    command = ListLabInventory(outpath=outpath)
    return self.execute_command(command=command, output_file_path=outpath)

  def execute_descriptor(self, model: str, outpath: str = None) -> dict:
    """Executes the Descriptor advisor sub command

        Args:
          model (str): model
          outpath (str, optional): output file path for the json file

        Returns:
          dict: Data representation of the json command output
        """
    if outpath is None:
      file_name = f"descriptor_{datetime.now().strftime('%Y-%m-%d')}.json"
      outpath = os.path.join(self._advisor_output_dir, file_name)
    command = Descriptor(model=model, outpath=outpath)
    return self.execute_command(command=command, output_file_path=outpath)

  def execute_properties(self, hwid: str, outpath: str = None) -> dict:
    """Executes the Properties advisor sub command

        Args:
            hwid (str): hwid
            outpath (str, optional): output file path for the json file
        Returns:
            dict: Data representation of the json command output
        """
    if outpath is None:
      hwid_file_name = "_".join(("_".join(hwid.split("-"))).split())
      properties_filename = f"properties_{hwid_file_name}.json"
      outpath = os.path.join(self._advisor_output_dir, properties_filename)
    command = Properties(hwid=hwid, outpath=outpath)
    return self.execute_command(command=command, output_file_path=outpath)
