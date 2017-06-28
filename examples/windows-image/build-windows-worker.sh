#!/bin/bash -e

# Load constants
source "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/constants.sh"

# Download ISOs
"$DIR/download.sh"

# Create temporary files
DATA=`mktemp -d`
ISO="$DIR/cache/data.iso"
TCWORKER=`mktemp`

# Run govendor sync
govendor sync

echo '### Building taskcluster-worker for host (for building image)'
go build -o "$TCWORKER" github.com/taskcluster/taskcluster-worker

echo '### Building taskcluster-worker for image (for use inside image)'
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -o "$DATA/taskcluster-worker.exe" \
  github.com/taskcluster/taskcluster-worker

echo '### Packaging data.iso'
cp "$DIR/data"/* "$DATA/"
7z x -o"$DATA/" "$VIRTIO_WIN_ISO"
genisoimage -vJrV DATA_VOLUME -input-charset utf-8 -o "$ISO" "$DATA"

echo "### Building $WORKER_IMAGE_NAME"
"$TCWORKER" qemu-build --boot "$INSTALLER_ISO" --cdrom "$ISO" --size "$DISKSIZE" \
  from-new "$DIR/machine.json" "$DIR/cache/$WORKER_IMAGE_NAME"

echo '### Removing temporary files'
rm -rf "$DATA"
rm -f "$ISO" "$TCWORKER"
