#!/bin/bash -e

# Load constants
source "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/constants.sh"

# Create temporary files
DATA=`mktemp -d`;
ISO=`mktemp`;
TCWORKER=`mktemp`;

echo '### Building taskcluster-worker for host (for building image)'
make build -C "$DIR/../.."
mv "$DIR/../../taskcluster-worker" "$TCWORKER"

echo '### Building taskcluster-worker for image (for use inside image)'
GOARCH="$IMAGE_GOARCH" make build -C "$DIR/../.."
mv "$DIR/../../taskcluster-worker" "$DATA/taskcluster-worker"

echo '### Packaging data.iso'
cp "$DIR/data"/* "$DATA/"
genisoimage -vJrV DATA_VOLUME -input-charset utf-8 -o "$ISO" "$DATA"

echo "### Building $SETUP_IMAGE_NAME"
"$TCWORKER" qemu-build --no-vnc --cdrom "$ISO" \
  from-image "$DIR/cache/$SETUP_IMAGE_NAME" "$DIR/cache/$WORKER_IMAGE_NAME"

echo '### Removing temporary files'
rm -rf "$DATA"
rm -f "$ISO" "$TCWORKER"
