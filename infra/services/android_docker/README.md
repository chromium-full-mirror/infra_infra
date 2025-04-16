The script here is used to manage the containers on android swarming bots.
By design, it's stateless and event-driven. It has a few modes of execution:
* `launch`: Ensures every locally connected android device has a running
          container. On the bots, it's called every 5 minutes via cron.
* `add_device`: Gives a device's container access to its device. Called every
              time a device appears/reappears on the system via udev.
* `list`: This simply prints the serials and their physical port nums. This
        does not touch or modify any containers or swarming bots on the machine.
        It only prints info to stdout. This can be useful since mapping devices
        to their port nums can be non-trivial, and we wouldn't want that logic
        duplicated by any service that needs to know the mapping.

It's intended to be run in conjucture with the android_docker container image.
More information on the image can be found [here](https://chromium.googlesource.com/infra/infra/+/HEAD/docker/docker_devices/README.md).


Launching the containers
--------------------------
Every 5 minutes, this script is invoked with the `launch` argument. This is
how the containers get spawned, shut down, and cleaned up. Essentially, it does
the following:
* Gracefully shutdowns each container if container uptime is too high (default
4 hours) or host uptime is too high (default 24 hours).
* Scans the USB bus for any connected android device and creates a container
for each one.


Adding a device to a container
--------------------------
On linux, docker limits containers to their set of resources via
cgroups ([v1](https://docs.kernel.org/admin-guide/cgroup-v1/index.html),
[v2](https://docs.kernel.org/admin-guide/cgroup-v2.html)), e.g. memory,
network, etc. However the device controller implementation changed from
[interface files in v1](https://docs.kernel.org/admin-guide/cgroup-v1/devices.html)
to [cgroup BPF in v2](https://docs.kernel.org/admin-guide/cgroup-v2.html#device-controller)
which doesn't have any interface files.

To support this change, we first use the docker container option
[`--device-cgroup-rule`](https://docs.docker.com/reference/cli/docker/container/run/#device-cgroup-rule)
to grant the container access to all devices of a given
[major number][major and minor nums]. This is because every time a device
reboots or resets, it momentarily disappears from the host. When this happens,
many things that uniquely identify the device change. This includes its
[dev and bus numbers](http://www.makelinux.net/ldd3/chp-13-sect-2.shtml), and
[minor number][major and minor nums]. But the major number likely remains
unchanged as it identifies the driver associated with the device which doesn't
change.

Then a custom [udev](https://linux.die.net/man/7/udev) rule is written so that
every time an android device appears on the host, the script is invoked with
`add_device` argument which adds the device to the container by creating a
[device node](https://linux.die.net/man/3/mknod) in the container's `/dev`
filesystem.

[major and minor nums]: https://www.makelinux.net/ldd3/chp-3-sect-2.shtml

Gracefully shutting down a container
--------------------------
To preform various forms of maintenance, the script here gracefully shuts down
containers and, by association, the swarming bot running inside them. This is
done by sending SIGTERM to the swarming bot process. This alerts the swarming
bot that it should quit at the next available opportunity (ie: not during a
test.)
In order to fetch the pid of the swarming bot, the script runs `lsof` on the
[swarming.lck](https://cs.chromium.org/chromium/infra/luci/appengine/swarming/doc/Bot.md?rcl=8b90cdd97f8f088bcba2fa376ce49d9863b48902&l=305)
file.


Fetching the docker image
--------------------------
When a new container is launched with an image that is missing
on a bot's local cache, it pulls it from the specified container registry. For
all bots, this is the gcloud registry
[chops-public-images-prod](https://pantheon.corp.google.com/gcr/images/chops-public-images-prod/global).
These images are world-readable, so no authentication with the registry is
required. Additionally, when new images are fetched, any old and unused images
still present on the machine will be deleted.


File locking
--------------------------
To avoid multiple simultaneous invocations of this service from stepping
on itself (there are a few race conditions exposed when a misbehaving device is
constantly connecting/disconnecting), parts of the script are wrapped in a mutex
via a flock. The logic that's protected includes:
* scanning the USB bus (wrapped in a global flock)
* any container interaction (wrapped in a device-specific flock)


Getting device list
--------------------------
py-libusb is used to fetch a list of locally connected devices, which is part
of this service's
[vpython spec](https://chromium.googlesource.com/infra/infra/+/beef78d6298a5b1b896dd801ce1c9fce775e8451/infra/services/android_docker/vpython_main.py#60).
This library wraps the [libusb system lib](http://www.libusb.org/) and provides
methods for fetching information about USB devices.


Deploying the script
--------------------------
The script and its dependencies are deployed as a CIPD package via puppet. The
package (infra/swarm_docker_tools/$platform) is continously built on this
[bot](https://ci.chromium.org/p/infra-internal/builders/luci.infra-internal.prod/infra-packager-linux-64).
Puppet deploys it to the relevant bots at
[these revisions](https://chrome-internal.googlesource.com/infra/puppet/+/94c1a63589e03a543035937b586dbf86ba5b2d3e/puppetm/etc/puppet/hieradata/cipd.yaml#417).
The canary pin affects bots on [chromium-swarm-dev](https://chromium-swarm-dev.appspot.com),
which the android testers on the [chromium.dev](https://luci-milo-dev.appspot.com/p/chromium/g/chromium.dev/console)
waterfall run tests against. If the canary has been updated, the bots look fine,
and the tests haven't regressed, you can proceed to update the stable pin.

The call sites of the script are also defined in puppet:
* [cron](https://chrome-internal.googlesource.com/infra/puppet/+/94c1a63589e03a543035937b586dbf86ba5b2d3e/puppetm/etc/puppet/modules/chrome_infra/templates/setup/docker/android/android_docker_cron.sh.erb)
* [udev](https://chrome-internal.googlesource.com/infra/puppet/+/94c1a63589e03a543035937b586dbf86ba5b2d3e/puppetm/etc/puppet/modules/chrome_infra/files/setup/docker/android_docker_udev)
