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

echo '### Packaging data.iso'
cp "$DIR/data"/* "$DATA/"
genisoimage -vJrV DATA_VOLUME -input-charset utf-8 -o "$ISO" "$DATA"

echo "### Building $SETUP_IMAGE_NAME"
echo 'This step opens a VNC sessions and requires you to:'
echo ' 1. Install operating system'
echo ' 2. Mount cdrom'
echo ' 3. Execute install-customize-image.sh'
"$TCWORKER" qemu-build --boot "$INSTALLER_ISO" --cdrom "$ISO" --size "$DISKSIZE" \
  from-new "$DIR/machine.json" "$DIR/cache/$SETUP_IMAGE_NAME"

echo '### Removing temporary files'
rm -rf "$DATA"
rm -f "$ISO" "$TCWORKER"
