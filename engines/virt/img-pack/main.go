package main

// github.com/pierrec/lz4

// Deals with compression and decompression of sparse raw files.
// Compress:   disk.raw -> disk.raw.tar.lz4 (tar contains disk.raw)
// Decompress: disk.raw.tar.lz4 disk.raw (fails if tar contains anything but disk.raw)
// See:
// https://godoc.org/github.com/pierrec/lz4
// https://golang.org/pkg/archive/tar/
// TODO: Benchmark lz4, zip. lzw, zlib, flate, bzip2, snappy, lzo
// Create sparse files with: truncate --size=8192M disk.img
func main() {

}
