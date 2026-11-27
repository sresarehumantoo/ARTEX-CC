package traffic

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/andybalholm/brotli"
)

const compressedPage = "<html><body>Welcome admin. root:x:0:0:root:/root:/bin/bash</body></html>"

func recordCompressed(t *testing.T, tr *Traffic, path, enc string, body []byte) {
	t.Helper()
	u, _ := url.Parse("http://api.example.com" + path)
	tr.recordViews(
		requestView{method: "GET", url: u, proto: "HTTP/1.1", header: http.Header{}},
		responseView{status: 200, header: http.Header{"Content-Type": {"text/html"}, "Content-Encoding": {enc}}, body: body},
		nil,
	)
}

func bodyHits(t *testing.T, tr *Traffic, term string) int {
	t.Helper()
	rows, _, err := tr.Page(PageQuery{Host: "api.example.com", Body: term, RespMin: -1, RespMax: -1}, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	return len(rows)
}

func TestCompressedBodyIndexEncodings(t *testing.T) {
	var gz, br, zl bytes.Buffer
	gw := gzip.NewWriter(&gz)
	io.WriteString(gw, compressedPage)
	gw.Close()
	bw := brotli.NewWriter(&br)
	io.WriteString(bw, compressedPage)
	bw.Close()
	zw := zlib.NewWriter(&zl)
	io.WriteString(zw, compressedPage)
	zw.Close()

	for _, tc := range []struct {
		enc  string
		body []byte
	}{{"gzip", gz.Bytes()}, {"br", br.Bytes()}, {"deflate", zl.Bytes()}} {
		t.Run(tc.enc, func(t *testing.T) {
			tr := openTestTraffic(t)
			if !tr.fts {
				t.Skip("no full-text index in this build")
			}
			recordCompressed(t, tr, "/"+tc.enc, tc.enc, tc.body)
			if n := bodyHits(t, tr, "root:x:0"); n != 1 {
				t.Fatalf("body search for text of a %s response: %d rows, want 1", tc.enc, n)
			}
		})
	}
}

// A compression bomb from a target must not expand past the index limit.
func TestDecodeBodyIsBounded(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(make([]byte, 4*maxIndexBody))
	gz.Close()
	out, ok := decodeBody(http.Header{"Content-Encoding": {"gzip"}}, buf.Bytes(), maxIndexBody)
	if !ok || len(out) != maxIndexBody {
		t.Fatalf("decodeBody returned %d bytes, ok=%v; want %d", len(out), ok, maxIndexBody)
	}
}

func TestTruncatedCompressedBodyIsIndexed(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	io.WriteString(gz, compressedPage)
	for i := range 4096 {
		fmt.Fprintf(gz, "row %d\n", i)
	}
	gz.Close()
	cut := buf.Bytes()[:buf.Len()/2]

	tr := openTestTraffic(t)
	if !tr.fts {
		t.Skip("no full-text index in this build")
	}
	recordCompressed(t, tr, "/cut", "gzip", cut)
	if n := bodyHits(t, tr, "root:x:0"); n != 1 {
		t.Fatalf("body search for text of a truncated gzip response: %d rows, want 1", n)
	}
}

func TestDecodeBodyKeepsTextBeforeBrotliCut(t *testing.T) {
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	io.WriteString(bw, compressedPage)
	for i := range 4096 {
		fmt.Fprintf(bw, "row %d\n", i)
	}
	bw.Close()
	out, ok := decodeBody(http.Header{"Content-Encoding": {"br"}}, buf.Bytes()[:buf.Len()/2], maxIndexBody)
	if !ok || !bytes.Contains(out, []byte("root:x:0")) {
		t.Fatalf("decodeBody on truncated br = (%d bytes, %v), want the text before the cut", len(out), ok)
	}
}

func TestIndexBodySkipsBinaryContentType(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	io.WriteString(gz, compressedPage)
	gz.Close()
	h := http.Header{"Content-Encoding": {"gzip"}, "Content-Type": {"image/png"}}
	if text, ok := indexBody(h, buf.Bytes()); ok || text != "" {
		t.Fatalf("indexBody on an image/png body = (%q, %v), want (\"\", false)", text, ok)
	}
}

func TestDecodeBodyRejectsStackedAndUnknown(t *testing.T) {
	for _, enc := range []string{"", "identity", "gzip, br", "zstd"} {
		if _, ok := decodeBody(http.Header{"Content-Encoding": {enc}}, []byte("x"), 10); ok {
			t.Errorf("decodeBody(%q) ok=true, want false", enc)
		}
	}
}
