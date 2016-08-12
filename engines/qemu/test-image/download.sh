#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

aws s3 cp "s3://public-qemu-images/test-images/tinycore-setup.tar.lz4" "$DIR/tinycore-setup.tar.lz4"
aws s3 cp "s3://public-qemu-images/test-images/tinycore-worker.tar.lz4" "$DIR/tinycore-worker.tar.lz4"
