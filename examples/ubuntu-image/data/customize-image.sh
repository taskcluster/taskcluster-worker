#!/bin/sh

# Disable customize-image.service
systemctl disable customize-image.service
rm /etc/systemd/system/customize-image.service
rm /opt/customize-image.sh

# Mount cdrom
mount /dev/cdrom /mnt

sh /mnt/setup.sh

# Unmount cdrom
umount /dev/cdrom
