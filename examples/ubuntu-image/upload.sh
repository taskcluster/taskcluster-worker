#!/bin/bash -e

# Load constants
source "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/constants.sh"

# We don't upload $SETUP_IMAGE_NAME as this file is the result of manually
# constructing the image and following the install instructions in the README.md
# So we'll rarely need to update this, it's mostly if there is breaking image
# format changes that we might have to.
echo "Skipping $SETUP_IMAGE_NAME as we don't change it"
#aws s3 cp "$DIR/cache/$SETUP_IMAGE_NAME" "s3://$S3_BUCKET/$S3_PREFIX/$SETUP_IMAGE_NAME"

aws s3 cp "$DIR/cache/$WORKER_IMAGE_NAME" "s3://$S3_BUCKET/$S3_PREFIX/$WORKER_IMAGE_NAME"
