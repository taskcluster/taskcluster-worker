#!/bin/bash -e

# Load constants
source "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/constants.sh"

# Create temporary files
DATA=`mktemp -d`;
ISO=`mktemp`;
TCWORKER=`mktemp`;
function finish {
  echo '### Removing temporary files'
  rm -rf "$DATA"
  rm -f "$ISO" "$TCWORKER"
}
trap finish EXIT

echo '### Building taskcluster-worker for host (for building image)'
make build -C "$DIR/../.."
mv "$DIR/../../taskcluster-worker" "$TCWORKER"

echo '### Extract Ubuntu ISO'
7z x -o"$DATA/" "$INSTALLER_ISO"

echo '### Set grub timeout'
#taken from https://github.com/fries/prepare-ubuntu-unattended-install-iso/blob/master/make.sh
sed -i -r 's/timeout\s+[0-9]+/timeout 1/g' "$DATA/isolinux/isolinux.cfg"

echo '### Add boot option'
# https://github.com/netson/ubuntu-unattended/blob/master/create-unattended-iso.sh
SEED_CHECKSUM=`md5sum "$DIR/data/worker.seed" | cut -f1 -d ' '`
sed -i "/label install/ilabel autoinstall\n\
  menu label ^Autoinstall Ubuntu Worker\n\
  kernel /install/vmlinuz\n\
  append file=/cdrom/custom-data/worker.seed \
    initrd=/install/initrd.gz auto=true priority=high \
    preseed/file=/cdrom/custom-data/worker.seed --" "$DATA/isolinux/txt.cfg"

echo '### Adding custom-data folder'
mkdir "$DATA/custom-data"

echo '### Building taskcluster-worker for image (for use inside image)'
GOARCH="$IMAGE_GOARCH" make build -C "$DIR/../.."
mv "$DIR/../../taskcluster-worker" "$DATA/custom-data/taskcluster-worker"

echo '### Packaging data.iso'
cp "$DIR/data"/* "$DATA/custom-data"
genisoimage -D -r -V "UBUNTU_INSTALLER" -cache-inodes -J -l \
  -b isolinux/isolinux.bin -c isolinux/boot.cat -no-emul-boot \
  -boot-load-size 4 -boot-info-table -o "$ISO" "$DATA"

echo "### Building $WORKER_IMAGE_NAME"
"$TCWORKER" qemu-build --boot "$ISO" --size "$DISKSIZE" \
  from-new "$DIR/machine.json" "$DIR/cache/$WORKER_IMAGE_NAME"
