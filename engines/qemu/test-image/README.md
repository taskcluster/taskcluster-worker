Test Image Build Process
========================

This folder contains two binary images:
  * `tinycore-setup.tar.lz4`, an image for easily rebuilding the test image,
  * `tinycore-worker.tar.lz4`, a test image running qemu-guest-tools on boot.

To the test the QEMU engine we need a virtual machine image that obtains an
IP using DHCP and runs `taskcluster-worker qemu-guest-tools` after boot.
The qemu-guest-tools are responsible for talking to the `MetaService` running on
the host (exposed using magic IP: `169.254.169.254`).

The qemu-guest-tools will execute task-specific command returned by the
`MetaService`, as well as upload logs to the `MetaService`.
The qemu-guest-tools is also long-polling the `MetaService` for actions, such as
upload an artifact, list a folder or start an interactive shell. Finally, the
qemu-guest-tools executes actions returned from the `MetaService`.

The interaction between qemu-guest-tools and `taskcluster-worker` running on the
host is obviously an important thing to test. And the interaction patterns
subject to rapid change during active development. Therefore, do we not only
ship a test image with the qemu-guest-tools installed and configured, but also
a setup image that enables us to quickly rebuild the test image.

Overhead of building the test image is significant enough that we don't want to
do it as part of the test case setup phase. The process also involves
cross-compiling `taskcluster-worker` for 32 bit Linux, so it might not be
super elegant to do as part of test setup.


Rebuilding the Test Image
-------------------------
As long as we have the `tinycore-setup.tar.lz4` image we can quickly rebuild
the test image by simply running `./rebuild.sh` from this folder.
This scripts requires `genisoimage` to be installed.

Note, the `tinycore-worker.tar.lz4` image is under revision control, and should
be commited after qemu-guest-tools have been updated.


Rebuilding the Setup Image
--------------------------
The `tinycore-setup.tar.lz4` image is a TinyCoreLinux 7.2 with a startup script
`/home/tc/.X.d/setup.sh` as follows:

```sh
#!/bin/sh -e

# Mount CD drive
sudo mount /dev/sr0

# Execute `setup.sh` from CD drive
sh /mnt/sr0/setup.sh
```

This means that booting the image with CD will run `setup.sh` from the CD, which
will then be able to install whatever it wants from the CD. This is what the
`./rebuild.sh` script uses to customize the virtual machine.

This image can be built by running the `taskcluster-worker qemu-build` command
with TinyCoreLinux ISO and a machine definition file `machine.json` file:

```js
{
  "uuid": "52bab607-10f1-4049-a0f8-ee4725cb715b",
  "network": {
    "device": "e1000",
    "mac": "aa:54:1a:30:5c:de"
  },
  "keyboard": {
    "layout": "en-us"
  }
}
```

The command for building the image is as follows, when doing this you may have
to follow TinyCoreLinux documentation on how to install and ensure that changes
are saved to disk when terminating.

```sh
./taskcluster-worker qemu-build \
  --size 1 \
  --boot tinycore.iso \
  from-new machine.json \
  tinycore-setup.tar.lz4
```
