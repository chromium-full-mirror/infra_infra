# servo lib

This module allows users to get correct servo-related variables for
flashing, specifically, dut-controls to be run before and after flashing
and the programmer argument.

These arguments mostly* depend on type of the servo in use. Thus, this
module provides utilities to parse the type of the servo.

\* - However, some boards require special arguments, configs for which may
be found
[here](https://source.corp.google.com/chromeos_public/chromite/lib/firmware/ap_firmware_config/).
