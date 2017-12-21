#!/bin/bash -e

# Load constants
source "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/constants.sh"

aws s3 cp "$DIR/cache/$WORKER_IMAGE_NAME" "s3://$S3_BUCKET/$S3_PREFIX/$WORKER_IMAGE_NAME"
