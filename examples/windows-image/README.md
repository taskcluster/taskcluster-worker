Windows 10 QEMU Image
=====================

This folder contains scripts and logic for building a QEMU image with Windows 10
for taskcluster-worker running QEMU engine.

This generally involves:
 1) Running `./download.sh` to fetch an Windows 10 ISO and virtio drivers
 2) Running `./build-windows-worker.sh`, which repackages the virtio ISO with
    `Autounattended.xml`, guest-tools and install scripts such that when the
    Windows 10 ISO is booted along with this ISO Windows will automatically
    be installed.

## Windows Activation
 1. Set timezone and date-time correct
 2. `cscript slmgr.vbs -skms-domain mozilla.com` (to make auto-discovery work)
 3. `cscript slmgr.vbs -ato` (activate windows)

(This isn't done as part of the image install scripts yet).
