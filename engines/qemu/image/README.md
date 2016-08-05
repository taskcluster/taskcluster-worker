Image Format Specification
==========================

An image file: `image.tar.lz4` is an lz4 compressed tar-ball containing the
following files:

  * `disk.img`, raw disk image (as sparse file).
  * `layer.qcow2`, qcow2 file with `disk.img` as backing file.
  * `machine.json`, JSON definition of machine configuration.

When constructing the tar-ball it's important to use GNU tar with the `-S`
option to ensure sparse file support.
