package traffic

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteBlobAtomic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := []byte("hello blob world")
	hash := "cafebabe"
	dest := filepath.Join(dir, hash+".bin")

	if err := writeBlobAtomic(dir, dest, hash, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("content mismatch: got %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o", info.Mode().Perm())
	}
}

func TestWriteBlobAtomicRaceWin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hash := "deadbeef"
	dest := filepath.Join(dir, hash+".bin")
	os.WriteFile(dest, []byte("existing"), 0o644)

	err := writeBlobAtomic(dir, dest, hash, []byte("newer"))
	if err != nil {
		t.Fatalf("expected no error when dest exists, got %v", err)
	}
}

func TestLimitedCaptureEmpty(t *testing.T) {
	t.Parallel()
	c := &limitedCapture{max: 100}
	c.record(nil)
	c.record([]byte{})
	if c.total != 0 || len(c.buf) != 0 || c.truncated {
		t.Fatalf("total=%d buf=%d trunc=%v", c.total, len(c.buf), c.truncated)
	}
}

func TestLimitedCaptureWithinLimit(t *testing.T) {
	t.Parallel()
	c := &limitedCapture{max: 100}
	c.record([]byte("hello"))
	c.record([]byte("world"))
	if c.total != 10 || string(c.buf) != "helloworld" || c.truncated {
		t.Fatalf("total=%d buf=%q trunc=%v", c.total, c.buf, c.truncated)
	}
}

func TestLimitedCaptureExceedsLimit(t *testing.T) {
	t.Parallel()
	c := &limitedCapture{max: 8}
	c.record([]byte("hello"))
	c.record([]byte("worldXYZ"))
	if c.total != 13 {
		t.Fatalf("total=%d, want 13", c.total)
	}
	if string(c.buf) != "hellowor" {
		t.Fatalf("buf=%q, want %q", c.buf, "hellowor")
	}
	if !c.truncated {
		t.Fatal("expected truncated")
	}
}

func TestLimitedCapturePastMaxNoMore(t *testing.T) {
	t.Parallel()
	c := &limitedCapture{max: 4}
	c.record([]byte("abcdef"))
	c.record([]byte("gh"))
	if c.total != 8 {
		t.Fatalf("total=%d", c.total)
	}
	if string(c.buf) != "abcd" {
		t.Fatalf("buf=%q", c.buf)
	}
	if !c.truncated {
		t.Fatal("expected truncated")
	}
}

func TestSpillRawDirEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	prev, hash := tr.spillRawDir(nil, 1024)
	if prev != nil || hash != "" {
		t.Fatalf("empty: prev=%v hash=%q", prev, hash)
	}
	prev, hash = tr.spillRawDir([]byte{}, 1024)
	if prev != nil || hash != "" {
		t.Fatalf("zero-len: prev=%v hash=%q", prev, hash)
	}
}

func TestSpillRawDirSmall(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	data := []byte("short payload")
	prev, hash := tr.spillRawDir(data, 1024)
	if hash != "" {
		t.Fatalf("small data should not spill, hash=%q", hash)
	}
	if !bytes.Equal(prev, data) {
		t.Fatalf("preview=%q", prev)
	}
}

func TestSpillRawDirLarge(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	data := bytes.Repeat([]byte("X"), 2048)
	prev, hash := tr.spillRawDir(data, 128)
	if hash == "" {
		t.Fatal("large data should spill to blob")
	}
	if len(prev) != 128 {
		t.Fatalf("preview len=%d, want 128", len(prev))
	}
}

func TestRecordRawTCPStreamBasic(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	now := time.Now().UTC()
	rec := RawTCPCapture{
		Host:      "10.10.10.5",
		Port:      22,
		StartedAt: now,
		EndedAt:   now.Add(time.Second),
		C2SBytes:  100,
		S2CBytes:  200,
		C2SData:   []byte("SSH-2.0-OpenSSH"),
		S2CData:   []byte("SSH-2.0-DropBear"),
		Truncated: false,
	}
	id, err := tr.RecordRawTCPStream(rec, DefaultSOCKS5PreviewBytes)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("empty id returned")
	}

	got, err := tr.GetRawTCPStream(id)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected non-nil stream")
	}
	if got.Host != "10.10.10.5" || got.Port != 22 {
		t.Fatalf("host=%q port=%d", got.Host, got.Port)
	}
	if got.Protocol != "tcp" {
		t.Fatalf("protocol=%q", got.Protocol)
	}
	if got.C2SBytes != 100 || got.S2CBytes != 200 {
		t.Fatalf("c2s=%d s2c=%d", got.C2SBytes, got.S2CBytes)
	}
	if got.DurationMs < 0 {
		t.Fatalf("duration_ms=%d", got.DurationMs)
	}
}

func TestRecordRawTCPStreamNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, err := tr.RecordRawTCPStream(RawTCPCapture{Host: "x", Port: 1}, 0)
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}

func TestRecordRawTCPStreamLargeBlob(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	bigC2S := bytes.Repeat([]byte("A"), DefaultSOCKS5PreviewBytes+100)
	bigS2C := bytes.Repeat([]byte("B"), DefaultSOCKS5PreviewBytes+200)
	rec := RawTCPCapture{
		Host:      "blobhost.test",
		Port:      443,
		StartedAt: time.Now().UTC(),
		EndedAt:   time.Now().UTC().Add(time.Second),
		C2SBytes:  int64(len(bigC2S)),
		S2CBytes:  int64(len(bigS2C)),
		C2SData:   bigC2S,
		S2CData:   bigS2C,
	}
	id, err := tr.RecordRawTCPStream(rec, DefaultSOCKS5PreviewBytes)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.GetRawTCPStream(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.C2SBlob == "" || got.S2CBlob == "" {
		t.Fatal("expected blob hashes for large data")
	}
	if len(got.C2SPreview) != DefaultSOCKS5PreviewBytes {
		t.Fatalf("c2s preview len=%d", len(got.C2SPreview))
	}
}

func TestRecordRawTCPStreamZeroTimes(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	rec := RawTCPCapture{
		Host: "zerot.test",
		Port: 80,
	}
	id, err := tr.RecordRawTCPStream(rec, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.GetRawTCPStream(id)
	if err != nil || got == nil {
		t.Fatalf("err=%v got=%v", err, got)
	}
	if got.DurationMs != 0 {
		t.Fatalf("duration_ms=%d, want 0", got.DurationMs)
	}
}

func TestGetRawTCPStreamNotFound(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	got, err := tr.GetRawTCPStream("nonexistent-id")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent")
	}
}

func TestGetRawTCPStreamNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, err := tr.GetRawTCPStream("any")
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}

func TestListRawTCPStreamsEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	streams, total, err := tr.ListRawTCPStreams(RawTCPFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(streams) != 0 {
		t.Fatalf("total=%d len=%d", total, len(streams))
	}
}

func TestListRawTCPStreamsWithRecords(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	for i := 0; i < 3; i++ {
		tr.RecordRawTCPStream(RawTCPCapture{
			Host:      "listhost.test",
			Port:      22,
			StartedAt: time.Now().UTC(),
			EndedAt:   time.Now().UTC(),
			C2SData:   []byte("hello"),
		}, 0)
	}
	tr.RecordRawTCPStream(RawTCPCapture{
		Host:      "other.test",
		Port:      443,
		StartedAt: time.Now().UTC(),
		EndedAt:   time.Now().UTC(),
	}, 0)

	streams, total, err := tr.ListRawTCPStreams(RawTCPFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("total=%d, want 4", total)
	}
	if len(streams) != 4 {
		t.Fatalf("len=%d, want 4", len(streams))
	}

	t.Run("host_filter", func(t *testing.T) {
		streams, total, err := tr.ListRawTCPStreams(RawTCPFilter{Host: "listhost.test"})
		if err != nil {
			t.Fatal(err)
		}
		if total != 3 || len(streams) != 3 {
			t.Fatalf("total=%d len=%d", total, len(streams))
		}
	})

	t.Run("port_filter", func(t *testing.T) {
		streams, total, err := tr.ListRawTCPStreams(RawTCPFilter{Port: 443})
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 || len(streams) != 1 {
			t.Fatalf("total=%d len=%d", total, len(streams))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		streams, total, err := tr.ListRawTCPStreams(RawTCPFilter{Limit: 2, Offset: 0})
		if err != nil {
			t.Fatal(err)
		}
		if total != 4 || len(streams) != 2 {
			t.Fatalf("total=%d len=%d", total, len(streams))
		}
		streams2, _, err := tr.ListRawTCPStreams(RawTCPFilter{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(streams2) != 2 {
			t.Fatalf("page2 len=%d", len(streams2))
		}
	})
}

func TestListRawTCPStreamsNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, _, err := tr.ListRawTCPStreams(RawTCPFilter{})
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}

func TestCountRawTCPStreamsEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	n, err := tr.CountRawTCPStreams("")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("count=%d", n)
	}
}

func TestCountRawTCPStreams(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	for i := 0; i < 3; i++ {
		tr.RecordRawTCPStream(RawTCPCapture{
			Host: "count.test", Port: 80,
			StartedAt: time.Now().UTC(), EndedAt: time.Now().UTC(),
		}, 0)
	}
	tr.RecordRawTCPStream(RawTCPCapture{
		Host: "other.test", Port: 80,
		StartedAt: time.Now().UTC(), EndedAt: time.Now().UTC(),
	}, 0)

	n, err := tr.CountRawTCPStreams("")
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("all count=%d, want 4", n)
	}

	n, err = tr.CountRawTCPStreams("count.test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("filtered count=%d, want 3", n)
	}
}

func TestCountRawTCPStreamsNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, err := tr.CountRawTCPStreams("")
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}
