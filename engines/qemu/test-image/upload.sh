#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "Skipping tinycore-setup.tar.lz4 as we don't change it"
# aws s3 cp "$DIR/tinycore-setup.tar.lz4" "s3://public-qemu-images/test-images/tinycore-setup.tar.lz4"
aws s3 cp "$DIR/tinycore-worker.tar.lz4" "s3://public-qemu-images/test-images/tinycore-worker.tar.lz4"
