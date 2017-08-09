#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Installer ISO
INSTALLER_S3_BUCKET='private-qemu-images'
INSTALLER_S3_KEY='en_windows_10_enterprise_version_1703_updated_march_2017_x64_dvd_10189290.iso'
INSTALLER_ISO="$DIR/cache/windows_10_x64.iso"

# virtio-win
VIRTIO_WIN_URL='https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.137-1/virtio-win.iso'
VIRTIO_WIN_ISO="$DIR/cache/virtio-win-0.1.137.iso"

DISKSIZE='20' # GB
IMAGE_GOARCH='amd64'

WORKER_IMAGE_NAME='windows-worker.tar.zst'
