package safenet

import (
	"bytes"
	"io"
	"net/netip"
)

func parseAddr(s string) (netip.Addr, error) {
	return netip.ParseAddr(s)
}

func newBytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
