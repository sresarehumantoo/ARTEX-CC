package traffic

import (
	"context"
	"testing"
)

func TestReadEvidenceNilTraffic(t *testing.T) {
	t.Parallel()
	var tr *Traffic
	err := tr.ReadEvidence(context.Background(), []string{"any-id"}, func(EvidenceExchange) error {
		t.Fatal("consume should not be called")
		return nil
	})
	if err == nil {
		t.Fatal("expected error for nil Traffic")
	}
}

func TestReadEvidenceContextCancellation(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"ev-0001", "evidence.test", "GET", "/", "https://evidence.test/",
		200, "text/html", 0, 4,
		"GET / HTTP/1.1\nHost: evidence.test\n", nil,
		"HTTP/1.1 200 OK\n", []byte("body"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := tr.ReadEvidence(ctx, []string{"ev-0001"}, func(EvidenceExchange) error {
		t.Fatal("consume should not be called on cancelled context")
		return nil
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestReadEvidenceNotFound(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	err := tr.ReadEvidence(context.Background(), []string{"nonexistent"}, func(EvidenceExchange) error {
		t.Fatal("consume should not be called")
		return nil
	})
	if err == nil {
		t.Fatal("expected error for nonexistent exchange")
	}
}

func TestReadEvidenceSuccess(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"ev-0002", "evidence.test", "POST", "/api/submit", "https://evidence.test/api/submit",
		201, "application/json", 15, 8,
		"POST /api/submit HTTP/1.1\nHost: evidence.test\n", []byte(`{"name":"test"}`),
		"HTTP/1.1 201 Created\nContent-Type: application/json\n", []byte(`{"id":42}`))

	var got EvidenceExchange
	err := tr.ReadEvidence(context.Background(), []string{"ev-0002"}, func(e EvidenceExchange) error {
		got = e
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "ev-0002" {
		t.Fatalf("ID = %q", got.ID)
	}
	if got.Method != "POST" {
		t.Fatalf("Method = %q", got.Method)
	}
	if got.Status != 201 {
		t.Fatalf("Status = %d", got.Status)
	}
	if got.URL != "https://evidence.test/api/submit" {
		t.Fatalf("URL = %q", got.URL)
	}
	if got.ContentType != "application/json" {
		t.Fatalf("ContentType = %q", got.ContentType)
	}
	if got.Request == nil {
		t.Fatal("Request reader is nil")
	}
	if got.Response == nil {
		t.Fatal("Response reader is nil")
	}
}

func TestReadEvidenceMultipleIDs(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)

	seedExchange(t, tr,
		"ev-m1", "multi.test", "GET", "/a", "https://multi.test/a",
		200, "text/html", 0, 2,
		"GET /a HTTP/1.1\n", nil,
		"HTTP/1.1 200 OK\n", []byte("aa"))
	seedExchange(t, tr,
		"ev-m2", "multi.test", "GET", "/b", "https://multi.test/b",
		200, "text/html", 0, 2,
		"GET /b HTTP/1.1\n", nil,
		"HTTP/1.1 200 OK\n", []byte("bb"))

	var ids []string
	err := tr.ReadEvidence(context.Background(), []string{"ev-m1", "ev-m2"}, func(e EvidenceExchange) error {
		ids = append(ids, e.ID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("got %d exchanges, want 2", len(ids))
	}
}

func TestReadEvidenceEmptyIDs(t *testing.T) {
	t.Parallel()
	tr := openTestTraffic(t)
	err := tr.ReadEvidence(context.Background(), nil, func(EvidenceExchange) error {
		t.Fatal("consume should not be called")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
