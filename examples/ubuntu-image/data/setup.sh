#!/bin/sh

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "$0" )" && pwd )"

# Wait for metadata service to be available
until curl -m 1 'http://169.254.169.254/engine/v1/ping'; do
  echo "Pinging..."
done

# Run install.sh sending logs to metadata service
bash "$DIR/install.sh" 2>&1 | "$DIR/taskcluster-worker" qemu-guest-tools post-log -

# Shutdown the system image is now customized
sleep 1
poweroff
