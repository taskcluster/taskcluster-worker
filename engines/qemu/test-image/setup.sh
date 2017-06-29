#!/bin/sh -e

# Install taskcluster-worker for the qemu-guest-tools
sudo cp /mnt/sr0/taskcluster-worker /opt/
sudo chmod +x /opt/taskcluster-worker

# Install a startup script to launch taskcluster-worker after X.org
cp /mnt/sr0/worker.sh /home/tc/.X.d/worker.sh
chmod +x /home/tc/.X.d/worker.sh

# Remove the existing setup script that called this script and unmount
rm -f /home/tc/.X.d/setup.sh
sudo umount /dev/sr0

# Run backup (otherwise changes on tinycore isn't saved)
filetool.sh -b

echo "Testing taskcluster-worker qemu-guest-tools" | DEBUG=* /opt/taskcluster-worker qemu-guest-tools post-log -
echo "Install completed" | curl -X POST -N --data-binary @- http://169.254.169.254/engine/v1/log

# Shutdown the system image is now customized
sudo poweroff
