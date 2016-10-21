#!/bin/bash -e

# Find location of the script no matter where it's located
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

curl "https://s3-us-west-2.amazonaws.com/public-qemu-images/test-images/tinycore-setup.tar.zst" > "$DIR/tinycore-setup.tar.zst"
curl "https://s3-us-west-2.amazonaws.com/public-qemu-images/test-images/tinycore-worker.tar.zst" > "$DIR/tinycore-worker.tar.zst"
