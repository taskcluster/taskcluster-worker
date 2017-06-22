#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Install URL
INSTALLER_URL='http://releases.ubuntu.com/16.04.2/ubuntu-16.04.2-server-amd64.iso'
INSTALLER_ISO="$DIR/cache/ubuntu-16.04.2-server-amd64.iso"
INSTALLER_SHA256='737ae7041212c628de5751d15c3016058b0e833fdc32e7420209b76ca3d0a535'

DISKSIZE='10' # GB
IMAGE_GOARCH='amd64'

WORKER_IMAGE_NAME='ubuntu-worker.tar.zst'

S3_BUCKET='public-qemu-images'
S3_PREFIX='repository/github.com/taskcluster/taskcluster-worker'
