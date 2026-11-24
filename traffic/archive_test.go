package traffic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeArchiveRuleIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []int64
		want []int64
	}{
		{"nil", nil, nil},
		{"empty", []int64{}, nil},
		{"all_zeroes", []int64{0, 0, 0}, nil},
		{"single", []int64{5}, []int64{5}},
		{"duplicates", []int64{3, 3, 1, 1}, []int64{1, 3}},
		{"mixed_signs", []int64{-1, 0, 2, -3, 4}, []int64{2, 4}},
		{"already_sorted", []int64{1, 2, 3}, []int64{1, 2, 3}},
		{"unsorted", []int64{9, 1, 5, 3}, []int64{1, 3, 5, 9}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeArchiveRuleIDs(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestUniqueArchiveHosts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, []string{}},
		{"empty_slice", []string{}, []string{}},
		{"duplicates", []string{"a.com", "b.com", "a.com"}, []string{"a.com", "b.com"}},
		{"mixed_case", []string{"A.COM", "a.com"}, []string{"a.com"}},
		{"whitespace", []string{" a.com ", "  b.com"}, []string{"a.com", "b.com"}},
		{"empty_strings", []string{"", " ", "a.com", ""}, []string{"a.com"}},
		{"sorted_output", []string{"z.com", "a.com", "m.com"}, []string{"a.com", "m.com", "z.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueArchiveHosts(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestImportArchiveNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	n, err := tr.ImportArchive(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
}

func TestExportHostsNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	n, err := tr.ExportHosts([]string{"example.com"}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
}

func TestExportHostsEmptyHosts(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	n, err := tr.ExportHosts(nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
}

func TestImportArchiveNoTrafficJSON(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	n, err := tr.ImportArchive(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
}

func TestImportArchiveUnsupportedVersion(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	dir := t.TempDir()
	snap := ArchiveSnapshot{Version: 99}
	raw, _ := json.Marshal(snap)
	if err := os.WriteFile(filepath.Join(dir, "traffic.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := tr.ImportArchive(dir)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestImportArchiveInvalidBlobHash(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	dir := t.TempDir()
	snap := ArchiveSnapshot{
		Version: archiveVersionV2,
		Blobs:   []string{"not-a-valid-hash"},
	}
	raw, _ := json.Marshal(snap)
	if err := os.WriteFile(filepath.Join(dir, "traffic.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := tr.ImportArchive(dir)
	if err == nil {
		t.Fatal("expected error for invalid blob hash")
	}
}

func TestImportArchiveBlobChecksumMismatch(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	dir := t.TempDir()

	fakeHash := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blobs", fakeHash+".bin"), []byte("wrong data"), 0o600); err != nil {
		t.Fatal(err)
	}
	snap := ArchiveSnapshot{
		Version: archiveVersionV2,
		Blobs:   []string{fakeHash},
	}
	raw, _ := json.Marshal(snap)
	if err := os.WriteFile(filepath.Join(dir, "traffic.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := tr.ImportArchive(dir)
	if err == nil {
		t.Fatal("expected error for blob checksum mismatch")
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"arc-0001", "archive.test", "GET", "/api/data", "https://archive.test/api/data",
		200, "application/json", 20, 50,
		"GET /api/data HTTP/1.1\nHost: archive.test\n", []byte(`{"q":"search"}`),
		"HTTP/1.1 200 OK\nContent-Type: application/json\n", []byte(`{"results":[]}`))

	seedExchange(t, tr,
		"arc-0002", "archive.test", "POST", "/api/submit", "https://archive.test/api/submit",
		201, "application/json", 30, 10,
		"POST /api/submit HTTP/1.1\nHost: archive.test\n", []byte(`{"name":"x"}`),
		"HTTP/1.1 201 Created\n", []byte(`{"id":1}`))

	exportDir := t.TempDir()
	n, err := tr.ExportHosts([]string{"archive.test"}, exportDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if n != 2 {
		t.Fatalf("exported %d, want 2", n)
	}

	raw, err := os.ReadFile(filepath.Join(exportDir, "traffic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snap ArchiveSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Version != archiveVersionV2 {
		t.Fatalf("version = %d", snap.Version)
	}
	if len(snap.Exchanges) != 2 {
		t.Fatalf("exchanges = %d", len(snap.Exchanges))
	}

	tr2 := openTestTraffic(t)
	imported, err := tr2.ImportArchive(exportDir)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported != 2 {
		t.Fatalf("imported %d, want 2", imported)
	}

	count, err := tr2.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count after import = %d, want 2", count)
	}

	imported2, err := tr2.ImportArchive(exportDir)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if imported2 != 0 {
		t.Fatalf("re-imported %d, want 0 (idempotent)", imported2)
	}
}

func TestImportArchiveV1(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	dir := t.TempDir()
	snap := ArchiveSnapshot{
		Version: archiveVersionV1,
		Exchanges: []ArchiveExchange{{
			ID: "v1-0001", TS: 1000, Host: "v1.test", Method: "GET",
			URLTemplate: "/", URL: "https://v1.test/", Status: 200,
			ContentType: "text/html", ReqLen: 0, RespLen: 4,
			ReqHead: "GET / HTTP/1.1\n", RespHead: "HTTP/1.1 200 OK\n",
			RespBody: []byte("body"),
		}},
	}
	raw, _ := json.Marshal(snap)
	if err := os.WriteFile(filepath.Join(dir, "traffic.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := tr.ImportArchive(dir)
	if err != nil {
		t.Fatalf("import v1: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported %d", n)
	}
}

func TestImportArchiveV2WithRewriteProvenance(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	dir := t.TempDir()
	snap := ArchiveSnapshot{
		Version: archiveVersionV2,
		Exchanges: []ArchiveExchange{{
			ID: "rw-0001", TS: 2000, Host: "rw.test", Method: "GET",
			URLTemplate: "/api/{id}", URL: "https://rw.test/api/42", Status: 200,
			ContentType: "application/json", ReqLen: 0, RespLen: 2,
			ReqHead: "GET /api/42 HTTP/1.1\n", RespHead: "HTTP/1.1 200 OK\n",
			RespBody: []byte("ok"), Rewritten: true, RewriteRuleIDs: []int64{10, 20},
		}},
	}
	raw, _ := json.Marshal(snap)
	if err := os.WriteFile(filepath.Join(dir, "traffic.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := tr.ImportArchive(dir)
	if err != nil {
		t.Fatalf("import v2: %v", err)
	}
	if n != 1 {
		t.Fatalf("imported %d", n)
	}
	rewritten, ruleIDs, err := tr.RewriteInfo("rw-0001")
	if err != nil {
		t.Fatal(err)
	}
	if !rewritten {
		t.Fatal("expected rewritten=true")
	}
	if len(ruleIDs) != 2 || ruleIDs[0] != 10 || ruleIDs[1] != 20 {
		t.Fatalf("ruleIDs = %v", ruleIDs)
	}
}

func TestExportImportWithBlobs(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	largeBody := make([]byte, maxInlineBody+100)
	for i := range largeBody {
		largeBody[i] = byte('A' + i%26)
	}
	sb := tr.spill(largeBody, "text/plain")
	if sb.hash == "" {
		t.Fatal("expected blob hash for large body")
	}

	sum := sha256.Sum256(largeBody)
	expectedHash := hex.EncodeToString(sum[:])

	db := tr.DB()
	_, err := db.Exec(`INSERT INTO exchanges(id,ts,host,method,url_template,url,status,content_type,req_len,resp_len,path)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		"blob-0001", 3000, "blob.test", "GET", "/big", "https://blob.test/big", 200, "text/plain", 0, len(largeBody), "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO exchange_bodies(id,req_head,req_body,req_blob,resp_head,resp_body,resp_blob)
		VALUES(?,?,?,?,?,?,?)`,
		"blob-0001", "GET /big HTTP/1.1\n", nil, nil, "HTTP/1.1 200 OK\n", sb.inline, expectedHash)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO blob_refs(hash,exchange_id) VALUES(?,?)`, expectedHash, "blob-0001")
	if err != nil {
		t.Fatal(err)
	}

	exportDir := t.TempDir()
	n, err := tr.ExportHosts([]string{"blob.test"}, exportDir)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if n != 1 {
		t.Fatalf("exported %d", n)
	}

	blobFile := filepath.Join(exportDir, "blobs", expectedHash+".bin")
	if _, err := os.Stat(blobFile); err != nil {
		t.Fatalf("blob file missing: %v", err)
	}

	tr2 := openTestTraffic(t)
	imported, err := tr2.ImportArchive(exportDir)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if imported != 1 {
		t.Fatalf("imported %d", imported)
	}

	f, size, err := tr2.Blob(expectedHash)
	if err != nil {
		t.Fatalf("blob lookup after import: %v", err)
	}
	defer f.Close()
	if size != int64(len(largeBody)) {
		t.Fatalf("blob size = %d, want %d", size, len(largeBody))
	}
}
