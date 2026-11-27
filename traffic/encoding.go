package traffic

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

// decodeBody undoes one Content-Encoding layer (gzip, deflate or br). The proxy
// stores bodies as they crossed the wire, so compressed text has to be decoded
// before it can be searched. Output is capped at limit because the target
// controls the input. A truncated or corrupt stream yields whatever decoded
// before the error.
func decodeBody(header http.Header, body []byte, limit int) (out []byte, ok bool) {
	enc := strings.ToLower(strings.TrimSpace(header.Get("Content-Encoding")))
	if enc == "" || enc == "identity" || len(body) == 0 || strings.Contains(enc, ",") {
		return nil, false
	}
	var zr io.Reader
	switch enc {
	case "gzip", "x-gzip":
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, false
		}
		zr = gz
	case "deflate":
		// HTTP deflate is zlib (RFC 9110); some servers send a raw deflate stream.
		if z, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
			zr = z
		} else {
			zr = flate.NewReader(bytes.NewReader(body))
		}
	case "br":
		zr = brotli.NewReader(bytes.NewReader(body))
	default:
		return nil, false
	}
	out, err := io.ReadAll(io.LimitReader(zr, int64(limit)))
	if err != nil && len(out) == 0 {
		return nil, false
	}
	return out, true
}

// indexBody returns the index text for a compressed body. When ok is false the
// text spill computed is kept.
func indexBody(header http.Header, body []byte) (text string, ok bool) {
	ct := header.Get("Content-Type")
	if isBinaryBody(ct, nil) {
		return "", false
	}
	dec, ok := decodeBody(header, body, maxIndexBody)
	if !ok {
		return "", false
	}
	if isBinaryBody(ct, dec) {
		return "", true
	}
	return string(dec), true
}

// DecodedResponse returns the stored head and decoded body of a compressed
// text response, with ok false for anything else.
func (t *Traffic) DecodedResponse(id string) (head string, body []byte, ok bool) {
	// Check the head first so uncompressed exchanges never load their blobs.
	var respHead string
	if err := t.db.QueryRow(`SELECT resp_head FROM exchange_bodies WHERE id=?`, id).Scan(&respHead); err != nil {
		return "", nil, false
	}
	_, _, headers := parseStoredResponseHead(respHead)
	h := http.Header{}
	h.Set("Content-Encoding", findHeaderValue(headers, "Content-Encoding"))
	ct := findHeaderValue(headers, "Content-Type")
	if h.Get("Content-Encoding") == "" || isBinaryBody(ct, nil) {
		return "", nil, false
	}
	_, _, _, respBody, err := t.loadFullExchange(id, "")
	if err != nil {
		return "", nil, false
	}
	dec, ok := decodeBody(h, respBody, maxIndexBody)
	if !ok || isBinaryBody(ct, dec) {
		return "", nil, false
	}
	return respHead, dec, true
}
