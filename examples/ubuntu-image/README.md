Ubuntu QEMU Image
=================

This folder contains scripts and logic for building a QEMU image with ubuntu
for taskcluster-worker running QEMU engine.

This generally involves:
 1) Running `./download.sh` to fetch an ubuntu ISO
 2) Running `./build-ubuntu-worker.sh`, which repackages the ISO with preseed
    files and guest-tools, and then builds the image automatically.
