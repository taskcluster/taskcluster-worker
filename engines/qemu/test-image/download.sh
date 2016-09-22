#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

aws s3 cp "s3://public-qemu-images/test-images/tinycore-setup.tar.zst" "$DIR/tinycore-setup.tar.zst"
aws s3 cp "s3://public-qemu-images/test-images/tinycore-worker.tar.zst" "$DIR/tinycore-worker.tar.zst"
