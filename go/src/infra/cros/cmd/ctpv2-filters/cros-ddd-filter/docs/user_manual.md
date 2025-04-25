# TTCP User manual

[TOC]

# TTCP Concepts


## Execution Context

The execution context is the environment the code under test is loaded into and executed.

A context is composed of many parts:


*   The hardware that executes the code under test.
*   The software loaded in addition to the code under test.
*   The configuration.
*   The test environment in which the hardware sits and can interact with.
*   etc.

For the rest of this document, the execution context will be referred to as context and we will only discuss its hardware aspect.


## Classes

A class is a set of contexts that can run a test. One specific type of class is the set of contexts that execute the test in an equivalent manner. This is also often called a test variant or an equivalence class.

Executing the same version of the code under test on any device belonging to the same equivalence class should always yield an equal result.


## Categories

A category is a set of classes. One type of category is the variant category of a test. Each class (variant) represents a unique way in which a test is executed. If a test passes on all classes of such a category and the category contains all the distinct classes a test could be executed on, we have strong evidence that the feature under test will behave as expected everywhere it will be used.


# Using the TTCP Solver

The TTCP solver is a CLI which, given an input of testing hardware requirements, will yield an output of devices wherein the test should run. In the future it is meant to be integrated in the scheduling infrastructure, so that users can instead just define the properties in the [scheduling config file](https://source.corp.google.com/chromeos_public/infra/suite_scheduler/generated_configs/suite_scheduler.ini), and the test runs should be consistently updated; however currently, this is only available manually, meaning that its output must be manually input in the same config file to be effective (and consistently updated as the lab changes).

The following example showcases how to use the TTCP solver to improve a suite scheduler config file; the improvements expected are to ensure the devices the tests are run in are representative of all the devices the test should run in.


## Examples

A test author needs to test a touchpad driver. He knows that each touchpad manufacturer has a very different process and as such each unique touchpad vendor needs to be tested as each presents different risks.


## Solver (AKA CLI)

The solver's source code is located [here](https://source.corp.google.com/chromeos_internal/src/config-internal/ttcp/). The process to use [the solver](https://source.corp.google.com/chromeos_internal/src/config-internal/ttcp/solve_expression;l=11) consists of 4 steps outlined below:



1. [How to set up prerequisites](#bookmark=id.4yiyvgov8stm)
2. [How to create a request](#bookmark=id.avjyvivavjlz)
3. [How to run the code](#bookmark=id.1ewzww4addl9)
4. [How to utilize the output](#bookmark=id.dj0htvb2spwz)


### How to set up prerequisites


*   You need to have the internal source tree of Chromeos. For the latest info on how to check out the code refer to [ChromeOs developer guide](https://chromium.googlesource.com/chromiumos/docs/+/main/developer_guide.md). A simple way to achieve this post-setup would be:

```shell
> cd ~/chromiumos
https://chrome-internal.googlesource.com/chromeos/config-internal/+/main/ttcp/docs/protos/ttcp_syntax_doc.md#ttcp_CategoryExpression
> repo init -u https://chrome-internal.googlesource.com/chromeos/manifest-internal -b main

> repo sync -j4

```


*   You also need to have the Nix package system installed. The latest info on installing it can be found at [Nixos Download](https://nixos.org/download.html) . We recommend installing it for single user; a simple way to achieve this is:

```shell
sh <(curl -L https://nixos.org/nix/install) --no-daemon
```

*   The tools need access to Gcloud services thus need credentials. The easiest manner is to store your credentials
on your file system.

```shell
gcloud auth application-default login
```

### How to Create a Request (Input to the solver)

**Recommended:**

TTCP provides a set of [predefined categories](https://chrome-internal.googlesource.com/chromeos/config-internal/+/main/ttcp/docs/datasets/named_ttcp_classes_and_categories.md). Currently TTCP does not allow users to define their own predefined categories that can be reused, this is a feature that will become available in the future.

The simplest request is to directly use one of the predefined categories, for example the category of variants based on the distinct vendors of the touchpads used in Chromebooks. This would correspond to the following JSON:

```json
{
    "name": "HWID:touchpad_field_vendor_id:distinct_values"
}
```


**Advanced:** \
TTCP requests are protobuf objects of the type [CategoryExpression](https://chrome-internal.googlesource.com/chromeos/config-internal/+/main/ttcp/docs/protos/ttcp_syntax_doc.md#ttcp_CategoryExpression) encoded in JSON. A power user can try constructing one of these by hand as needed, though we recommend most users just use the one provided.


### How to Run the Code

First we change the directory to the root directory of TTCP

```shell
> cd ~/chromiumos/src/config-internal/ttcp
```

And we run the request through the solver, by running the following command

```shell
> ./solve_expression --variants "{\"name\" : \"HWID:touchpad_field_vendor:distinct_values\"}" --inventoryswarming
```

> **⚠  Note**
>
> The --inventoryswarming defines the set of devices available to run tests as the whole set of devices available within Skylab.

This will output the result to stdout. To save the result to the file /tmp/result.json

```shell
> ./solve_expression --variants "{\"name\" : \"HWID:touchpad_field_vendor:distinct_values\"}" --inventoryswarming --outpath /tmp/result.json
```

For all the available options of the CLI

```shell
> ./solve_expression -h
```

### How to Utilize the Output (Translating the Response)

The output from the solver will be a protobuf object [SolvedCategory](https://source.corp.google.com/chromeos_internal/src/config-internal/ttcp/protos/ttcp/solver/solver.proto;l=16) JSON encoded.

```json
{
    // ​​The expression field is the TTCP request that produced this result.
    "expression": {
        "Body": {
            "Name": "touchpad_field_vendor:distinct_values"
        }
    },
   // The flat_expression is a simplified version of the request that contains no
   // combinatorial categories or categories/classes references by name.
    "flat_expression": {
        "classes": [
            ...
        ]
    },
    // The classes field contains the computed classes.
    "classes": [
        {
	     // expression is the logical expression that defines which devices belong or
 	     // not to this variant/class.
            "expression": {
                "name": "touchpad_field_vendor_class:06cb",
                "expression": {
                    "Operator": {
                        "Property": {
                            "property_path": "touchpad_field_vendor",
                            "Condition": {
                                "StrEqual": "06cb"
                            }
                        }
                    }
                }
            },
	     // Devices is the list of devices from the inventory that belong to the
     // variant. In the previous examples the inventory used was all the devices
            // available in satlab.
            "devices": {
                "AKEMI-AOKF C5B-C6C-D5D-56S-U6A-A9C": {
                    "device_id": "AKEMI-AOKF C5B-C6C-D5D-56S-U6A-A9C"
                },
                ...
            },
	     // The legacy solution defines the minimal set of models that include devices
     // identified in the device field. It also defines the minimal set of boards
     // that includes the previous set of models
            "legacy_solution": [
                {
                    "board": "OCTOPUS",
                    "models": [
                        "BLORB",
                        ...
                    ]
                },
                ...
            ]
        },
        ...
    ]
}

```
## Adjusting Configs

We can use the boards and model information from the results to generate more accurate configs.

For example, if the “Translated Response” gave 3 groups of equivalent board/models:

```json
{
    "expression":      ... ,
    "flat_expression": ... ,
    "classes": [
        {
            "expression": ... ,
            "devices":    ... ,
            "legacy_solution": [
                {
                    "board": "BRYA",
                    "models": [
                        "BANSHEE",
			    "FELWINTER",
                        "PIRIKA"
                    ]
                }
            ]
        },

        {
            "expression": ... ,
            "devices":    ... ,
            "legacy_solution": [
                {
                    "board": "BRYA",
                    "models": [
                        "MITHRAX",
			    "REDRIX"
                    ]
                },
                {
                    "board": "HANA",
                    "models": [
                        "HANA"
                    ]
                }

            ]
        },
        {
            "expression": ... ,
            "devices":    ... ,
            "legacy_solution": [
                {
                    "board": "NAUTILUS",
                    "models": [
                        "NAUTILUSLTE"
                    ]
                },
                {
                    "board": "OCTOPUS",
                    "models": [
                        "FLEEX"
                    ]
                },
		   ...
            ]
        }
    ]
}
```

Based on the above, the user should try to find models of each variant that have the most availability in the applicable pool, and create a new suite config, for example:

```python
def example_config():
    """Defines the config.

    Returns: ScheduleConfig object.
    """
    return config_gen.create_config(
        name = "TTCPExample",
        suite = "my-lovely-suite",
        contacts = ["bard@google.com"],
        launch_profile = "new_build",
        branches = ["canary"],
        models_list = ["banshee","redrix", "nautiluslte", ], # banshee Group1, redrix Group2, nautiluslte Group3
        qs_account = "legacypool-suites",
        pool = "DUT_POOL_QUOTA",
        timeout_mins = 60 * 30,
        only_successful_build_required = True,
        run_via_cft = True,
    )
```

> **⚠  Note**
>
> Using boards and models to target variants is a best effort mechanism. Targeting via board/model does not mean that at each run the variant will be hit, but with each test run the probability of the variant being tested becomes increasingly probable.

# Advisor

The advisor is a TTCP utility that provides features to help design variants (such as getting information on [lab inventory] [device properties] [hardware ID metadata]). For the moment the set of features that are available is small, however they are intended to grow substantially as we improve the system.

To see  all the available command of the advisor

```shell
> ./advisor -h
```

To see the help for a particular command

```shell
> ./advisor <command_name> -h
```

## Lab inventory

To see the available set of distinct devices in the swarming fleet

```shell
> ./advisor list_lab_inventory
```

This will display the list of all the different HWID of the devices available in the fleet


## Device Properties

To see the properties of a particular device

```shell
> ./advisor properties --hwid 'WOOMAX-NAMM D5B-J3D-E5E-R2B-46Q'
```

## HWID metadata for a model

To see the all the variables that define a particular HWID model

```shell
> ./advisor descriptor --model WOOMAX
```

# Advanced Guide


> **⚠  Note**
>
>This section is specifically for more complex expressions/uses of TTCP. It is not required reading.


## Combinatorial Categories

There will be use cases where an engineer wants a combination of categories, such as “all unique fingerprint sensors on unique touchpads”. (Or all unique CPU and GPU combinations)

To make things easier we will write the TTCP expression in a file the content will be:

```json
{
   "value": {
       "name": "cross category of CPU and GPU",
       "combinatorial": {
           "subcategories": [
               {
                   "name": "HWID:gpu_field:distinct_values_and_absent"
               },
               {
                   "name": "HWID:cpu_field:distinct_values"
               }
           ]
       }
   }
}
```

This file is already present in the TTCP source code as docs/examples/user\_guide\_example\_3.json

```shell
> ./solve_expression --variantsfile "$PWD/docs/examples/user_guide_example_3.json" --inventoryswarming
```

## Sparse Categories.

Sparse categories are used when a test is sensitive to two different categories but the two categories do not have a combined effect.

```json
{
   "value": {
       "name": "cross category of CPU and GPU",
       "union": {
           "subcategories": [
               {
                   "name": "gpu_field:distinct_values_and_absent"
               },
               {
                   "name": "cpu_field:distinct_values"
               }
           ]
       }
   }
}
```

This file is already present in the TTCP source code as `docs/examples/user_guide_example_4.json`

```shell
> ./solve_expression --variantsfile "$PWD/docs/examples/user_guide_example_4.json" --inventoryswarming
```

## Class filter

A TTCP result might produce classes that do not have a single device to test on. This could for two reasons:



*   No device of that variant was never built
*   No device of that variant is available in the provided inventory.

As such, TTCP has an option to filter the variants into testable and untestable.

To get only the testable variants use the option `"–classfilter onlyTestable"`

```shell
> ./solve_expression --variantsfile "$PWD/docs/examples/user_guide_example_4.json" --inventoryswarming --classfilter onlyTestable
```

To get only the untestable variants use the option `"–classfilter onlyUntestable"`

```shell
> ./solve_expression --variantsfile "$PWD/docs/examples/user_guide_example_4.json" --inventoryswarming --classfilter onlyUntestable
```

## Opt-In & Opt-Out

There are cases one might not want to compute the variants for all the devices in the provided inventory.

For example one might want to compute the variants for the devices we are currently working on releasing but exclude the low touch variants.

An opt-in file contains the TTCP class that describes all the devices currently being worked on. In this example we declare that all the devices belonging to the boards `brya` and `octopus` are being worked on.

```json
{
   "value": {
       "name": "leading devices",
       "expression": {
           "property": {
               "propertyPath": "board",
               "str_in_set": {
                   "values": [
                       "BRYA",
                       "OCTOPUS"
                   ]
               }
           }
       }
   }
}
```

An opt-out file contains the TTCP class that describes all the low touch devices for the boards `brya` and `octopus` . In this example it is the device belonging to the models `skolas`, `taeko` and `vortininja`.

```json
{
   "value": {
       "name": "leading devices",
       "expression": {
           "property": {
               "propertyPath": "model",
               "str_in_set": {
                   "values": [
                       "SKOLAS",
                       "TAEKO",
                       "VORTININJA"
                   ]
               }
           }
       }
   }
}
```

The two files exist already in the TTCP repo as `docs/examples/user_guide_example_5_optin.json` and `docs/examples/user_guide_example_5_optin.json`

So we can compute the previous request with the command:

```shell
> ./solve_expression \
      --variantsfile "$PWD/docs/examples/user_guide_example_4.json" \
      --inventoryswarming \
      --classfilter onlyUntestable \
      --optinfile "$PWD/docs/examples/user_guide_example_5_optin.json" \
      --optoutfile "$PWD/docs/examples/user_guide_example_5_optout.json"
```

# User defined classes and categories

Users can define classes and categories that can be reused by referencing them by name in
classes, categories or test metadata.

An example is the file `datasets\static\common.json`.

For new classes and categories, they need to be stored in a json file that is a serialized `ttcp.collection` object. They can be added in an existing one or a new one, the only constraint is that they need to be in the `datasets` directory but not in the `dataset/generated directory`. The  `dataset/generated directory` is reserved for autogenerate collections, any manual change in the directory will be overwritten when the collections are regenerated.

## Generating readable equivalence class names

Users can define reportable categories to generate readable equivalence class names (user defined categories only).
These names can help users quickly identify the class vs scanning and interpreting
the long output of the solution classes.

### Output examples
#### Solver Service
The generated readable name is stored in the ```readableHumanName``` field of the solver service output.
```
{
  "schedulingUnitOptions": [
    {
      "publishKeys":
        [
          {
            "keyValues":
              {
                "eqcCategoryExpression": "{"name": "WifiBtChipset_Soc_Kernel"}",
                "eqcHash": "10846165061846201093",
                "readableHumanName": "Kabylake__INTEL_THP2_AC9260__5.10"
              },
            "subject"   : "3D"
          }
        ]
    }
  ]
}
```

#### solve_expression CLI

The generated readable name is stored in the ```name``` field in the ```classes``` struct of the solver service output.

```
"classes": [
        {
            "expression": {...},
            "targets": {...},
            "legacy_solutions": [...],
            "name": "Kabylake__INTEL_THP2_AC9260__5.10"
        },
        {
            "expression": {...},
            "targets": {...},
            "legacy_solutions": [...],
            "name": "Stoney Ridge__RTL8822CE__5.10"
        },
]
```

## Configuration

There are two parts to enabling readable name reporting for a user-defined categories, which are configured
in the ```datasets\static\<*>.json``` files

1. Create the reportable category definitions
2. Add the definition name to the user-defined category

### Creating reportable category definitions

Reportable categories are defined in the ``reportableCategories`` field of ```datasets\static\<*>.json``` file.

Note that the order of the ``categories`` elements determines the ordering of the generated '__' delimited name string.

The syntax for reportable categories:

```
{
    "reportCategories": {
        <reportCategory_id>: {
            "categories": [
                {
                    "value": {
                        "property": <property_field>,
                        "overrides": [
                            {
                                "match": <regex_match_rule>,
                                "replace": <replacement_string_value>
                            },
                            <override ... N>
                        ]
                    }
                },
                {
                    "name": <existing category>
                }
                <category ... N>
            ]
        },
        <reportCategory ... N>
    }
}
```

#### Example
```
    "reportCategories": {
        "wirelessSoc": {
            "categories": [
                {
                    "value": {
                        "property": "dlm:soc",
                        "overrides": [
                            {
                                "match": "Kabylake.*",
                                "replace": "Kabylake"
                            },
                            {
                                "match": "Apollolake.*",
                                "replace": "Apollolake"
                            },
                            {
                                "match": "Tigerlake.*",
                                "replace": "Tigerlake"
                            }
                        ]
                    }
                }
            ]
        }
    }
```

#### Reusing report categories
Once defined, a report category can be reused in building additional report categories with the ``name`` field

Example
```
"reportCategories": {
        "wifiBtChipset:soc:kernelVersion": {
            "categories": [
                {
                    "name": "wirelessSoc"
                },
                {
                    "name": "wirelessWifBtChipset"
                },
                {
                    "value": {
                        "property": "image:_kernel_version"
                    }
                }
            ]
        }
    }

```
### Add reporting category to the user-defined category
Once the reporting category has been defined, it then can be added to a user-defined category using the ``reportCategory`` field

#### Example
```
    "categories": {
        "Wireless:Wifi_Soc": {
            "name": "Wireless:Wifi_Soc",
            "description": "Category with unique Wifi SOCs",
            <...>
            "reportCategory": "wirelessSoc"

```

A full example can be found in the ```datasets\static\wifi_bluetooth.json```
# Limitations


## Automatically Scheduling Testing

Currently the tooling and guide requires users to manually call the CLI, and make the suite-scheduler configs. The automated portion of this will come in CTPv2/TTCP integration later in 2023.


## TestTracker Integration

Manual test plans and results are recorded in [TestTracker](https://g3doc.corp.google.com/company/teams/testtracker/new/index.md?cl=head) which we have no plans to integrate with at this time.


## Cost Reduction

As this implementation is manual, it’s not the intent that we would see any specific cost reduction. Cost right-sizing is expected after MVP when teams start to implement TTCP in their test scheduling which may or may not reduce cost depending on whether the testing need was correctly calibrated and maintained before TTCP.


## Test Metadata

The test metadata definitions are not yet completed and will not be defined in this guide; they are expected to evolve as we progress.


# Glossary

**[Board](https://moma.corp.google.com/glossary?entity=/g/t36yvkhr&q=chromeos%20board) **


    A group of ChromeOS devices (models) that have similar hardware, but where individual models for a board may vary in minor ways (e.g. screen size).

Reference:

**[Model](https://moma.corp.google.com/glossary?entity=%2Fg%2F0n_xryz1&hq=type%3Aglossary&q=label%3A%22crosinfra%22)**


    Chrome OS device that is unique in the market. A model typically maintains the major hardware components of its parent board but may vary in minor elements of one or more of: physical design, OEM, or ODM.

Reference:

**Class/Variant**


    A class is a group of devices that share a component in their design (e.g.: same wifi chip, or same screen), such that if a test targeting that device component is run in one, it should have the same results if you ran it in any of the others in that group. The terms class and variant are used interchangeably.

**Category**


    A category is a group of classes/variants that execute the code in an comparable manner.

**[CLI](https://en.wikipedia.org/wiki/Command-line_interface)**


    A command-Line Interface is a means of interacting with a device or computer program with commands from a user or client, and responses from the device or program, in the form of lines of text.

**Code Under Test**


    Code under test is the code a specific test stimulates and measures the outputs of.

**Coverage**


    The coverage of a test is how much a specific test correctly checks the behavior it was written to check. For instance, if a test was written to check whether wifis A and B behaved as expected given a specific scenario, the coverage is how effective that test is in simulating the condition to ensure the behavior is as expected.


    There are many different ways to mathematically measure coverage but in the context of TTCP coverage that measure is specifically: on how many of the variants was the test run.

**Driver**


    A driver is a program which allows users to control a particular type of hardware device that is attached to your computer.

**Execution context**


    Execution context is the place where the code is executed and is described by all the parameters that can influence the execution of the code.

**Inventory**


    In the context of TTCP, an inventory is the set of devices available for testing

**[SKU](https://moma.corp.google.com/glossary?entity=/g/11fzf40887&q=sku)**


    A Stock Keeping Unit is a distinct type of item for sale, such as a product or service, and all attributes associated with the item type that distinguish it from other item types. In this context you can think of it as a specific hardware component in the device.

**[BOM](https://moma.corp.google.com/glossary?entity=/g/11fz9xzxkr&q=bom)**


    A Bill of Materials is a list of parts, assemblies, and components. A collection of stock-keeping units (SKUs).

**[HWID](https://moma.corp.google.com/glossary?entity=/g/11pcwdqtx0&q=hwid)**


    The Hardware ID Is a unique identifier for each Chrome device SKU


    Reference:

**[HWID DB ](https://chromium.googlesource.com/chromiumos/platform/factory/+/HEAD/py/hwid/README.md)**


    The Hardware ID Database is the dataset that allows to translate a BOM to and from a HWID

**Metadata**

Metadata is data that is attached to an entity and describes the item.

**Under/Over-Testing**


    Under-Testing -  Under-testing is when the tests you have run are not comprehensively covering (See: Coverage) the expected behavior in all circumstances. Under-testing usually occurs when a test doesn’t run on all devices it needs to (e.g.: not running a UI test on all relevant screens)


    Over-Testing - Over-testing occurs when you run a test on more devices than required to achieve coverage (See: Coverage). For instance if devices A and B produce equal behavior and you test in both, you are over-testing.

**Properties**


    Within the scope of TTCP, a property is a set of inputs which a user defines to determine what components of a device they want to test on. An example of a property is a HWID (See: HWID). Currently only HWIDs are supported as properties, but in the future we intend to also support Boxster, among others.

**Scheduling**


    Scheduling is the act of selecting a suite (See: Suite) of tests to be run on specific devices. Generally a test can be scheduled preemptively (by defining it in [this configuration file](https://source.corp.google.com/chromeos_public/infra/suite_scheduler/generated_configs/suite_scheduler.ini)) at a specific cadence or ad-hoc (by using a variety of methods, including a [CLI](go/crosfleet)).

**Suite**


    A group of tests. Specifically it is literally just some tests grouped together and run together during scheduling.

**Target**

	A target is defined as a device or set of devices on which a test will run.

**Test**


    Specifically, tests here are integration tests, run on ChromeOS. These are not unit tests or browser tests

**Test Plan**


    A test plan is a detailed definition of where a test should run. Specifically it can be considered metadata relevant to the run of a test.

**TestTracker**


    A Google developed test management system used to create and define test suites and cases as well as record test results.


    Reference: go/testtracer-doc

**Touchpad**


    A touchpad is a pointing device that allows you to control the cursor or mouse pointer to select text, icons, files, and more. It is operated by dragging your finger across the flat surface of the touchpad, and the mouse cursor moves in the same direction.
