#!/bin/sh
cp -r /cdrom/custom-data /target/tmp/custom-data
chroot /target /bin/bash --login /tmp/custom-data/install.sh | /cdrom/custom-data/taskcluster-worker qemu-guest-tools post-log -
rm -rf /target/tmp/custom-data
