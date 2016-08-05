#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
DATA=`mktemp -d`;
ISO=`mktemp`;

echo "Building taskcluster-worker for host (for building image)"
go build -o "$DIR/../../../taskcluster-worker" \
  github.com/taskcluster/taskcluster-worker;

echo "Building taskcluster-worker for i386 (for use in image)"
CGO_ENABLED=0 GOARCH=386 go build -o "$DATA/taskcluster-worker" \
  github.com/taskcluster/taskcluster-worker;

echo "Packaging data.iso"
cp "$DIR/setup.sh" "$DATA/setup.sh";
cp "$DIR/worker.sh" "$DATA/worker.sh";
genisoimage -vJrV DATA_VOLUME -input-charset utf-8 -o "$ISO" "$DATA";

echo "Building image"
"$DIR/../../../taskcluster-worker" qemu-build --no-vnc --cdrom "$ISO" \
  from-image "$DIR/tinycore-setup.tar.lz4" "$DIR/tinycore-worker.tar.lz4";

echo "Removing temporary files"
rm -rf "$DATA";
rm -f "$ISO";
