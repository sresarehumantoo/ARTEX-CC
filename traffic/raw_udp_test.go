package traffic

import (
	"bytes"
	"testing"
	"time"
)

func TestRecordRawUDPDatagramBasic(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	rec := RawUDPCapture{
		AssociationID: "assoc-001",
		Host:          "10.10.10.5",
		Port:          53,
		Direction:     "c2s",
		Timestamp:     time.Now().UTC(),
		Payload:       []byte("DNS query payload"),
	}
	id, err := tr.RecordRawUDPDatagram(rec, DefaultSOCKS5PreviewBytes)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("empty id returned")
	}

	got, err := tr.GetRawUDPDatagram(id)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected non-nil datagram")
	}
	if got.Host != "10.10.10.5" || got.Port != 53 {
		t.Fatalf("host=%q port=%d", got.Host, got.Port)
	}
	if got.Direction != "c2s" {
		t.Fatalf("direction=%q", got.Direction)
	}
	if got.Protocol != "udp" {
		t.Fatalf("protocol=%q", got.Protocol)
	}
	if got.AssociationID != "assoc-001" {
		t.Fatalf("assocID=%q", got.AssociationID)
	}
	if got.Size != int64(len(rec.Payload)) {
		t.Fatalf("size=%d, want %d", got.Size, len(rec.Payload))
	}
	if !bytes.Equal(got.Preview, rec.Payload) {
		t.Fatalf("preview mismatch")
	}
}

func TestRecordRawUDPDatagramS2C(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	rec := RawUDPCapture{
		AssociationID: "assoc-002",
		Host:          "8.8.8.8",
		Port:          53,
		Direction:     "s2c",
		Timestamp:     time.Now().UTC(),
		Payload:       []byte("DNS response"),
	}
	id, err := tr.RecordRawUDPDatagram(rec, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.GetRawUDPDatagram(id)
	if err != nil || got == nil {
		t.Fatalf("err=%v got=%v", err, got)
	}
	if got.Direction != "s2c" {
		t.Fatalf("direction=%q", got.Direction)
	}
}

func TestRecordRawUDPDatagramNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, err := tr.RecordRawUDPDatagram(RawUDPCapture{
		Host: "x", Port: 1, Direction: "c2s",
		Timestamp: time.Now(), Payload: []byte("x"),
	}, 0)
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}

func TestRecordRawUDPDatagramInvalidDirection(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	_, err := tr.RecordRawUDPDatagram(RawUDPCapture{
		Host: "x.test", Port: 53, Direction: "invalid",
		Timestamp: time.Now(), Payload: []byte("x"),
	}, 0)
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}
}

func TestRecordRawUDPDatagramLargeBlob(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	big := bytes.Repeat([]byte("U"), DefaultSOCKS5PreviewBytes+500)
	rec := RawUDPCapture{
		AssociationID: "assoc-big",
		Host:          "big.test",
		Port:          9999,
		Direction:     "c2s",
		Timestamp:     time.Now().UTC(),
		Payload:       big,
	}
	id, err := tr.RecordRawUDPDatagram(rec, DefaultSOCKS5PreviewBytes)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.GetRawUDPDatagram(id)
	if err != nil || got == nil {
		t.Fatalf("err=%v got=%v", err, got)
	}
	if got.Blob == "" {
		t.Fatal("expected blob hash for large payload")
	}
	if len(got.Preview) != DefaultSOCKS5PreviewBytes {
		t.Fatalf("preview len=%d", len(got.Preview))
	}
}

func TestRecordRawUDPDatagramZeroTimestamp(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	rec := RawUDPCapture{
		Host:      "zerot.test",
		Port:      53,
		Direction: "c2s",
		Payload:   []byte("x"),
	}
	id, err := tr.RecordRawUDPDatagram(rec, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.GetRawUDPDatagram(id)
	if err != nil || got == nil {
		t.Fatalf("err=%v got=%v", err, got)
	}
	if got.Timestamp <= 0 {
		t.Fatalf("expected auto-filled timestamp, got %d", got.Timestamp)
	}
}

func TestGetRawUDPDatagramNotFound(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	got, err := tr.GetRawUDPDatagram("nonexistent-udp-id")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent")
	}
}

func TestGetRawUDPDatagramNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, err := tr.GetRawUDPDatagram("any")
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}

func TestListRawUDPDatagramsEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	dgrams, total, err := tr.ListRawUDPDatagrams(RawUDPFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(dgrams) != 0 {
		t.Fatalf("total=%d len=%d", total, len(dgrams))
	}
}

func TestListRawUDPDatagramsWithRecords(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	for i := 0; i < 3; i++ {
		tr.RecordRawUDPDatagram(RawUDPCapture{
			AssociationID: "assoc-list",
			Host:          "list.test",
			Port:          53,
			Direction:     "c2s",
			Timestamp:     time.Now().UTC(),
			Payload:       []byte("query"),
		}, 0)
	}
	tr.RecordRawUDPDatagram(RawUDPCapture{
		AssociationID: "assoc-list",
		Host:          "list.test",
		Port:          53,
		Direction:     "s2c",
		Timestamp:     time.Now().UTC(),
		Payload:       []byte("response"),
	}, 0)
	tr.RecordRawUDPDatagram(RawUDPCapture{
		AssociationID: "assoc-other",
		Host:          "other.test",
		Port:          8080,
		Direction:     "c2s",
		Timestamp:     time.Now().UTC(),
		Payload:       []byte("other"),
	}, 0)

	dgrams, total, err := tr.ListRawUDPDatagrams(RawUDPFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("total=%d, want 5", total)
	}
	if len(dgrams) != 5 {
		t.Fatalf("len=%d, want 5", len(dgrams))
	}

	t.Run("assoc_filter", func(t *testing.T) {
		d, n, err := tr.ListRawUDPDatagrams(RawUDPFilter{AssociationID: "assoc-list"})
		if err != nil {
			t.Fatal(err)
		}
		if n != 4 || len(d) != 4 {
			t.Fatalf("total=%d len=%d", n, len(d))
		}
	})

	t.Run("host_filter", func(t *testing.T) {
		d, n, err := tr.ListRawUDPDatagrams(RawUDPFilter{Host: "other.test"})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || len(d) != 1 {
			t.Fatalf("total=%d len=%d", n, len(d))
		}
	})

	t.Run("port_filter", func(t *testing.T) {
		d, n, err := tr.ListRawUDPDatagrams(RawUDPFilter{Port: 8080})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || len(d) != 1 {
			t.Fatalf("total=%d len=%d", n, len(d))
		}
	})

	t.Run("direction_filter", func(t *testing.T) {
		d, n, err := tr.ListRawUDPDatagrams(RawUDPFilter{Direction: "s2c"})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 || len(d) != 1 {
			t.Fatalf("total=%d len=%d", n, len(d))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		d1, n, err := tr.ListRawUDPDatagrams(RawUDPFilter{Limit: 2, Offset: 0})
		if err != nil {
			t.Fatal(err)
		}
		if n != 5 || len(d1) != 2 {
			t.Fatalf("page1: total=%d len=%d", n, len(d1))
		}
		d2, _, err := tr.ListRawUDPDatagrams(RawUDPFilter{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(d2) != 2 {
			t.Fatalf("page2 len=%d", len(d2))
		}
		d3, _, err := tr.ListRawUDPDatagrams(RawUDPFilter{Limit: 2, Offset: 4})
		if err != nil {
			t.Fatal(err)
		}
		if len(d3) != 1 {
			t.Fatalf("page3 len=%d", len(d3))
		}
	})
}

func TestListRawUDPDatagramsNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, _, err := tr.ListRawUDPDatagrams(RawUDPFilter{})
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}

func TestCountRawUDPDatagramsEmpty(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	n, err := tr.CountRawUDPDatagrams("", "")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("count=%d", n)
	}
}

func TestCountRawUDPDatagrams(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	for i := 0; i < 3; i++ {
		tr.RecordRawUDPDatagram(RawUDPCapture{
			AssociationID: "count-assoc",
			Host:          "count.test",
			Port:          53,
			Direction:     "c2s",
			Timestamp:     time.Now().UTC(),
			Payload:       []byte("x"),
		}, 0)
	}
	tr.RecordRawUDPDatagram(RawUDPCapture{
		AssociationID: "count-other",
		Host:          "other.test",
		Port:          53,
		Direction:     "c2s",
		Timestamp:     time.Now().UTC(),
		Payload:       []byte("y"),
	}, 0)

	n, err := tr.CountRawUDPDatagrams("", "")
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("all count=%d, want 4", n)
	}

	n, err = tr.CountRawUDPDatagrams("count-assoc", "")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("assoc count=%d, want 3", n)
	}

	n, err = tr.CountRawUDPDatagrams("", "other.test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("host count=%d, want 1", n)
	}

	n, err = tr.CountRawUDPDatagrams("count-assoc", "count.test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("assoc+host count=%d, want 3", n)
	}
}

func TestCountRawUDPDatagramsNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	_, err := tr.CountRawUDPDatagrams("", "")
	if err == nil {
		t.Fatal("expected error for nil traffic")
	}
}
