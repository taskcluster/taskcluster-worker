#!/bin/sh -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Disable auto-updates of any kind
apt-get purge -y unattended-upgrades
rm /etc/apt/apt.conf.d/10periodic

# Copy in customize-image.service
cp "$DIR/customize-image.service" /etc/systemd/system/
chmod 644 /etc/systemd/system/customize-image.service

cp "$DIR/customize-image.sh" /opt/
chmod +x /opt/customize-image.sh

# Enable customize-image.service
systemctl enable customize-image.service
echo 'customize-image.service installed'
