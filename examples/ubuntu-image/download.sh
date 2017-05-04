#!/bin/bash -e

# Load constants
source "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/constants.sh"

if ! [ -f "$INSTALLER_ISO" ]; then
  echo '### Downloading Installer ISO'
  curl -L "$INSTALLER_URL" > "$INSTALLER_ISO"
fi

echo '### Checking sha256sum'
echo "$INSTALLER_SHA256 $INSTALLER_ISO" | sha256sum -c -

if ! [ -f "$DIR/cache/$SETUP_IMAGE_NAME" ]; then
  echo "### Downloading $SETUP_IMAGE_NAME"
  curl -L "https://s3-us-west-2.amazonaws.com/$S3_BUCKET/$S3_PREFIX/$SETUP_IMAGE_NAME" > "$DIR/cache/$SETUP_IMAGE_NAME"
fi

echo "Not downloading $WORKER_IMAGE_NAME as it is easy to rebuild"
