#!/bin/sh -e

# Start the guest-tools worker
export DEBUG='*'
sudo /opt/taskcluster-worker qemu-guest-tools > /home/tc/worker.log 2> /home/tc/worker.log

# Kill without synchronizing
sudo poweroff -n -f
