Ubuntu QEMU Image
=================

This folder contains scripts and logic for building a QEMU image with ubuntu
for taskcluster-worker running QEMU engine.

This generally involves:
 1) Installing Ubuntu
 2) Setting up `taskcluster-worker qemu-guest-tools` to run after start-up

We do this in two steps, first we create an `ubuntu-setup.tar.zst` image that
when booted will mount `/dev/cdrom` and execute `setup.sh` from the CD drive.
This way we can tweak the contents of the ISO mounted as `/dev/cdrom`, and
derive a new image without having to install ubuntu from scratch.

Creating `ubuntu-setup.tar.zst`
-------------------------------
First run the `download.sh` to download images, then run `build-ubuntu-setup.sh`
and install ubuntu, mount `/dev/sr1` and run `install-customize-image.sh` from
the CD drive. This installs a service that will run after start-up, mount
`/dev/cdrom` and execute `setup.sh`.

Note. **this involves manually steps**, installation of ubuntu isn't automated.
For consistency we enter username/password as:

```
username: ubuntu
password: ubuntu
```

Creating `ubuntu-worker.tar.zst`
-------------------------------
Once we have the `ubuntu-setup.tar.zst` building an image we can use with the
worker is as easy as running `build-ubuntu-setup.sh`.

Note: there's currently no automated testing that image building is successful.
