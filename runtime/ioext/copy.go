package ioext

import "io"

// Copy from dst to src returning bytes written, write error werr and read error
// rerr. This is similar to io.Copy except users can distinguish between read and
// write errors.
func Copy(dst io.Writer, src io.Reader) (written int64, werr, rerr error) {
	b := make([]byte, 32*1024)

	for {
		nr, er := src.Read(b)
		if er != nil && er != io.EOF {
			if nr > 0 {
				dst.Write(b[0:nr]) // Ignore errors here as we had a read error first
			}
			rerr = er
			break
		}
		if nr > 0 {
			nw, ew := dst.Write(b[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				werr = ew
				break
			}
			if nr != nw {
				werr = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
	}
	return
}
