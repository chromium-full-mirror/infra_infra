# Copyright 2021 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
from collections import OrderedDict

from recipe_engine import recipe_api
from recipe_engine.recipe_api import Property
# Windows command helpers
from . import offline_winpe_customization as offwinpecust
from . import online_windows_customization as onwincust
from . import windows_iso_customization as winiso
from . import sources
from . import helper
from PB.recipes.infra.windows_image_builder import windows_image_builder as wib
from PB.recipes.infra.windows_image_builder import sources as src_pb
from PB.recipes.infra.windows_image_builder import dest as dest_pb

# Customization to Builder mapping
CUST_BUILDER = {
    'offline_winpe_customization': 'wib-wim-customization',
    'online_windows_customization': 'wib-windows-customization',
    'windows_iso_customization': 'wib-windows-customization',
}

BUILDERS = set(CUST_BUILDER.values())


class WindowsPSExecutorAPI(recipe_api.RecipeApi):
  """API for using Windows PowerShell scripts."""

  def __init__(self, envvars, **kwargs):
    super(WindowsPSExecutorAPI, self).__init__(**kwargs)
    self._scripts = self.resource
    self._workdir = ''
    self._sources = None
    self._configs_dir = None
    self._customizations = []
    self._executable_cust = []
    self._config = wib.Image()
    self._try_job = False
    if envvars.MAX_CUST_BATCH_SIZE:
      self._max_cust_batch_size = int(envvars.MAX_CUST_BATCH_SIZE)
    else:
      # Batch 1 cust per job by default
      self._max_cust_batch_size = 1

  def init(self, try_job=False):
    """ init initializes all the dirs and sub modules required."""
    self._try_job = try_job
    with self.m.step.nest('Initialize the config engine'):
      self._sources = sources.Source(self.m.path.cache_dir / 'Pkgs', self.m)

      self._configs_dir = self.m.path.cleanup_dir / 'configs'
      helper.ensure_dirs(self.m.file, [self._configs_dir])

  def init_customizations(self, config):
    """ init_customizations initializes the given config and returns
    list of customizations

    Args:
      * config: wib.Image proto config
    """
    # ensure that arch is specified in the image
    if config.arch == wib.Arch.ARCH_UNSPECIFIED:
      raise self.m.step.StepFailure('Missing arch in config')

    arch = wib.Arch.Name(config.arch).replace('ARCH_', '').lower()

    custs = []  # list of customizations from customizations.py
    # initialize all customizations
    for cust in config.customizations:
      if cust.WhichOneof('customization') == 'offline_winpe_customization':
        custs.append(
            offwinpecust.OfflineWinPECustomization(
                image=config,
                cust=cust,
                arch=arch,
                scripts=self._scripts,
                configs=self._configs_dir,
                module=self.m,
                source=self._sources,
                try_job=self._try_job,
            ))
      if cust.WhichOneof('customization') == 'online_windows_customization':
        custs.append(
            onwincust.OnlineWindowsCustomization(
                image=config,
                cust=cust,
                arch=arch,
                scripts=self._scripts,
                configs=self._configs_dir,
                module=self.m,
                source=self._sources,
                try_job=self._try_job,
            ))
      if cust.WhichOneof('customization') == 'windows_iso_customization':
        custs.append(
            winiso.WinISOCustomization(
                image=config,
                cust=cust,
                arch=arch,
                scripts=self._scripts,
                configs=self._configs_dir,
                module=self.m,
                source=self._sources,
                try_job=self._try_job,
            ))

    return custs

  def process_customizations(self, custs, ctx, inputs=()):
    """ process_customizations pins all the volatile srcs and generates
    canonnical configs.

    Args:
      * custs: List of customizations from customization.py
      * ctx: dict containing the context for the customization
      * inputs: List of inputs that are required

    Returns list of customizations in order that they were processed
    """
    with self.m.step.nest('Process the customizations'):
      resolved_cust = []
      while len(custs) != len(resolved_cust):
        pinnable_cust = []
        for cust in custs:
          if cust not in resolved_cust:
            if cust.pinnable(ctx):
              pinnable_cust.append(cust)
        if pinnable_cust:
          self.pin_customizations(pinnable_cust, ctx)
          self.gen_canonical_configs(pinnable_cust)
          # If the cust is online, inject cache upload
          # request for drives
          for cust in pinnable_cust:
            cust_type = cust.customization().WhichOneof('customization')
            if cust_type == 'online_windows_customization':
              cust.inject_cache_upload(inputs)
          ctx = self.update_context(pinnable_cust, ctx)
          resolved_cust.extend(pinnable_cust)
        else:
          unpinned_custs = ''
          for cust in custs:
            if cust not in resolved_cust:
              unpinned_custs += cust.id + '<br>'
              unpins = '<ul>'
              for pin in cust.unpinnable(ctx):
                unpins += '<li>{}</li>'.format(pin.local_src)
              unpins += '</ul> <br>'
          reason = 'Cannot pin custs [<br> {} <br>].'.format(unpinned_custs)
          raise self.m.step.StepFailure(
              'Cyclical dependency?: {}'.format(reason))
      return resolved_cust

  def pin_customizations(self, customizations, ctx):
    """ pin_customizations pins all the sources in the customizations

    Args:
      * customizations: List of Customizations object from customizations.py
      * ctx: dict containing the context for the customization
    """
    for cust in customizations:
      with self.m.step.nest('Pin resources from {}'.format(cust.name())):
        cust.pin_sources(ctx)

  def trim_uploads(self, customizations):
    """ trim_uploads removes the user specified uploads from a config.
    """
    for cust in customizations:
      cust.remove_upload_dests()

  def update_context(self, custs, ctx):
    """ update_context returns an updated dict with all the contexts
    updated

    Args:
    * custs: List of customizations from customization.py
    * ctx: Current context

    Returns updated context dict
    """
    for cust in custs:
      ctx.update(cust.context)
    return ctx

  def gen_canonical_configs(self, customizations):
    """ gen_canonical_configs strips all the names in the config and returns
    individual configs containing one customization per image.

    Example:
      Given an Image

        Image{
          arch: x86,
          name: "windows10_x86_GCE",
          customizations: [
            Customization{
              OfflineWinPECustomization{
                name: "winpe_networking"
                image_dest: GCSSrc{
                  bucket: "chrome-win-wim"
                  source: "rel/win10_networking.wim"
                }
                ...
              }
            },
            Customization{
              OfflineWinPECustomization{
                name: "winpe_diskpart"
                image_src: Src{
                  gcs_src: GCSSrc{
                    bucket: "chrome-win-wim"
                    source: "rel/win10_networking.wim"
                  }
                }
                ...
              }
            }
          ]
        }

      Writes two configs: windows10_x86_GCE-winpe_networking.cfg with

        Image{
          arch: x86,
          name: "",
          customizations: [
            Customization{
              OfflineWinPECustomization{
                name: ""
                image_dest: GCSSrc{
                  bucket: "chrome-win-wim"
                  source: "rel/win10_networking.wim"
                }
                ...
              }
           }
          ]
        }

      and windows10_x86_GCE-winpe_diskpart.cfg with

        Image{
          arch: x86,
          name: "",
          customizations: [
            Customization{
              OfflineWinPECustomization{
                name: ""
                image_src: Src{
                  gcs_src: GCSSrc{
                    bucket: "chrome-win-wim"
                    source: "rel/win10_networking.wim"
                  }
                }
                ...
              }
            }
          ]
        }

      to disk, calculates the hash for each config and sets the key for each
      of them. The strings representing name of the image, customization,...
      etc,. are set to empty before calculating the hash to maintain the
      uniqueness of the hash.

    Args:
      * customizations: List of Customizations object from customizations.py
    """
    for cust in customizations:
      # create a new image object, with same arch and containing only one
      # customization
      canon_image = wib.Image(
          arch=cust.image().arch, customizations=[cust.get_canonical_cfg()])
      name = cust.name()
      # write the config to disk
      cfg_file = self._configs_dir.joinpath('{}-{}.cfg'.format(
          cust.image().name, name))
      self.m.file.write_proto(
          'Write config {}'.format(cfg_file),
          cfg_file,
          canon_image,
          codec='BINARY')
      # estimate the unique hash for the config (identifier for the image built
      # by this config)
      key = self.m.file.file_hash(cfg_file)
      cust.set_key(key)
      # save the config to disk as <key>.cfg
      key_file = self._configs_dir.joinpath('{}.cfg'.format(key))
      self.m.file.copy('Copy {} to {}'.format(cfg_file, key_file), cfg_file,
                       key_file)

    return customizations

  def filter_executable_customizations(self, customizations):
    """ filter_executable_customizations generates a list of customizations
    that need to be executed.

    Args:
      * customizations: List of Customizations object from customizations.py
    """
    exec_customizations = []
    for cust in customizations:
      if cust.needs_build:
        exec_customizations.append(cust)
    return exec_customizations

  def download_all_packages(self, custs):
    """ download_all_packages downloads all the packages referenced by given
    custs.

    Args:
      * custs: List of Customizations object from customizations.py
    """
    for cust in custs:
      with self.m.step.nest('Download resources for {}'.format(cust.name())):
        cust.download_sources()

  def execute_customizations(self, custs):
    """ Executes the windows image builder user config.

    Args:
      * custs: List of Customizations object from customizations.py
    """
    with self.m.step.nest('execute config {}'.format(custs[0].image().name)):
      for cust in custs:
        cust.execute_customization()

  def get_executable_configs(self, custs):
    """ get_executable_configs returns a list of images that can be executed at
    this time.

    image is generated after determining what customizations can be executed as
    part of the image. A list of keys representing the customizations to be
    executed are also returned with the image.

    Args:
      * custs: List of customizations to be processed.
    Returns a dict mapping builder name to the image-key_list tuple
    """
    # executions is a dict that will be returned
    executions = OrderedDict()
    for builder in BUILDERS:
      # filter all the customizations for the builder
      builder_custs = [
          cust for cust in custs if CUST_BUILDER[cust.type] == builder
      ]
      # Get the image, key_list tuple for the given builder
      images = self.gen_executable_configs(builder_custs)
      if images:
        executions[builder] = images
    return executions

  def gen_executable_configs(self, custs):
    """ gen_executable_configs generates wib.Image configs that can be executed.

    Given a list of custs that can be run on a builder. Generates wib.Image
    proto configs making sure that they can be executed independently. Some
    customizations are dependent on others for inputs and can only be executed
    if the dependent input is generated in the same wib.Image proto or is
    already available.

    Args:
      * custs: list of customization objects from customizations.py that can be
      executed on the same builder

    Returns a list of tuples containing config and set of customization hash
    that can be executed at the time
    """
    # Dict mapping the customization object key to itself
    key_cust_map = OrderedDict()
    for cust in custs:
      # Update the key map
      key_cust_map[cust.get_key()] = cust

    # List of customization keys that are already processed
    processed_cust = []
    # List of images. This will be returned by the method
    images = []
    for key, cust in key_cust_map.items():
      # First find a customization that can be executed without any dependency
      if key not in processed_cust and cust.executable():
        # image is list of customizations that can be executed on the builder
        image = [key]
        # ensure that image contains customizations for the same arch
        arch = cust.arch
        # Add the image to the list of images to return
        images.append(image)
        # record the customization key that is already being processed
        processed_cust.append(key)
        # get the list of outputs that are generated by this customization. This
        # will be used to determine if we can execute the following
        # customizations
        outputs = [self._sources.get_url(op) for op in cust.outputs]
        for nkey, node in key_cust_map.items():
          # If we can execute this customization then add it to the image
          if node.arch == arch and nkey not in processed_cust and \
          node.executable(
              inputs=outputs):
            # Add the node to list of processed_cust
            processed_cust.append(nkey)
            # Add the node to the image
            image.append(nkey)
            # Extend the outputs to include the ones generated by this node
            outputs.extend([self._sources.get_url(op) for op in node.outputs])
          if len(image) >= self._max_cust_batch_size:
            break

    # Convert all the images to wib.Image configs and return back
    configs = []
    for image in images:
      config = wib.Image()
      config.name = '_'.join(
          sorted(
              set([key_cust_map[cust_key].image().name for cust_key in image])))
      keys = set()
      for cust_key in image:
        customization = key_cust_map[cust_key].customization()
        # Include all the image names this config represents
        config.customizations.append(customization)
        keys.add(cust_key)
      # Add an image name and arch
      config.arch = key_cust_map[image[0]].image().arch
      configs.append((config, keys))

    return configs
