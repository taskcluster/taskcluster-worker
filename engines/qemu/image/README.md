Image Format Specification
==========================

An image file: `image.tar.lz4` is an lz4 compressed tar-ball containing the
following files:

  * `disk.img`, raw disk image (as sparse file).
  * `layer.qcow2`, qcow2 file with `disk.img` as backing file.
  * `machine.json`, JSON definition of machine configuration.

When constructing the tar-ball it's important to use GNU tar with the `-S`
option to ensure sparse file support.


Rebuilding the Test Image
-------------------------
Test cases for the QEMU engine uses the `tinycore-worker.tar.lz4` image,
this is TinyCoreLinux 7.2 with `taskcluster-worker` installed at
`/opt/taskcluster-worker` and a start-up script `/home/tc/.X.d/worker.sh`
as follows:

```sh
#!/bin/sh -e

# Start the guest-tools worker
sudo /opt/taskcluster-worker qemu-guest-tools > /home/tc/worker.log 2> /home/tc/worker.log

# Kill without synchronizing
sudo poweroff -n -f
```

This image can be built by creating a `data.iso` file containing
taskcluster-worker statically built for x86 (`CGO_ENABLED=0 GOARCH=386 go build`)
and the `worker.sh` script above. Then with a `machine.json` file as follows:
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

We can run `taskcluster-worker qemu-build` and go through the installation
process for TinyCoreLinux, this involves booting and using the VNC display to
download the TinyCore app for installing the OS, as well as mounting the cdrom
and copying the files onto the system.

```sh
./taskcluster-worker qemu-build \
  --size 1 \
  --boot tinycore.iso \
  --cdrom data.iso \
  from-new machine.json \
  tinycore-worker.tar.lz4
```
