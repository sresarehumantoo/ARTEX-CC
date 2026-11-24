package store

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestDeleteAssetsByHostMock(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec("DELETE FROM assets WHERE domain .* OR ip").
		WithArgs("example.com").
		WillReturnResult(sqlmock.NewResult(0, 3))

	n, err := s.DeleteAssetsByHost("example.com")
	if err != nil {
		t.Fatalf("DeleteAssetsByHost: %v", err)
	}
	if n != 3 {
		t.Errorf("deleted = %d, want 3", n)
	}
}

func TestDeleteAssetsByHostMock_Empty(t *testing.T) {
	t.Parallel()
	s, _ := newRegexpMockStore(t)

	_, err := s.DeleteAssetsByHost("")
	if err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestDeleteAssetsByHostMock_Trimming(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectExec("DELETE FROM assets").
		WithArgs("example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	n, err := s.DeleteAssetsByHost("  EXAMPLE.COM.  ")
	if err != nil {
		t.Fatalf("DeleteAssetsByHost: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted = %d, want 1", n)
	}
}

func assetSelectColumns() []string {
	return []string{
		"id", "type", "company_id", "task_ids",
		"domain", "root_domain", "ip", "c_segment", "port",
		"icp", "bound_domains",
		"record_type", "record_value",
		"bundle_id", "app_name", "category", "app_description", "app_icp",
		"url", "service_type", "service_name",
		"favicon_mmh3", "status_code", "content_length", "page_title",
		"technologies", "method", "extra",
		"created_at", "last_seen", "updated_at",
	}
}

func TestListAssets_ByID(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM assets WHERE id = ").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows(assetSelectColumns()).AddRow(
			int64(42), "root_domain", nil, "{1}",
			"example.com", "example.com", "", nil, nil,
			"", "{}",
			"", "{}",
			"", "", "", "", "",
			"", "", "",
			"", nil, nil, "",
			"{}", "", []byte("{}"),
			now, now, now,
		))

	assets, total, err := s.ListAssets(ctx(), tools.AssetFilter{ID: 42})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(assets) != 1 {
		t.Fatalf("len(assets) = %d, want 1", len(assets))
	}
	if assets[0].Type != "root_domain" {
		t.Errorf("type = %q, want root_domain", assets[0].Type)
	}
}

func TestListAssets_ByIDs(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	now := time.Now()

	rows := sqlmock.NewRows(assetSelectColumns())
	for _, id := range []int64{1, 2} {
		rows.AddRow(
			id, "ip", nil, "{1}",
			"", "", "10.0.0.1", nil, nil,
			"", "{}",
			"", "{}",
			"", "", "", "", "",
			"", "", "",
			"", nil, nil, "",
			"{}", "", []byte("{}"),
			now, now, now,
		)
	}

	mock.ExpectQuery("SELECT .+ FROM assets WHERE id IN").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(rows)

	assets, total, err := s.ListAssets(ctx(), tools.AssetFilter{IDs: []int64{1, 2}})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(assets) != 2 {
		t.Fatalf("len(assets) = %d, want 2", len(assets))
	}
}

func TestListAssets_WithFilters(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	now := time.Now()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs("root_domain", int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery("SELECT .+ FROM assets").
		WithArgs("root_domain", int64(5), 50, 0).
		WillReturnRows(sqlmock.NewRows(assetSelectColumns()).AddRow(
			int64(10), "root_domain", nil, "{5}",
			"test.com", "test.com", "", nil, nil,
			"", "{}",
			"", "{}",
			"", "", "", "", "",
			"", "", "",
			"", nil, nil, "",
			"{}", "", []byte("{}"),
			now, now, now,
		))

	assets, total, err := s.ListAssets(ctx(), tools.AssetFilter{Type: "root_domain", TaskID: 5})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(assets) != 1 {
		t.Fatalf("len(assets) = %d, want 1", len(assets))
	}
}

func TestInsertAsset_RootDomain(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets.*type.*root_domain").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type:   "root_domain",
		Domain: "example.com",
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 99 {
		t.Errorf("id = %d, want 99", id)
	}
}

func TestInsertAsset_IP(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets.*type.*ip").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(100)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type: "ip",
		IP:   "192.168.1.1",
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 100 {
		t.Errorf("id = %d, want 100", id)
	}
}

func TestInsertAsset_OtherServiceNoURL(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets.*type.*service").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type:        "service",
		ServiceName: "ftp",
		Port:        21,
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 101 {
		t.Errorf("id = %d, want 101", id)
	}
}

func TestInsertAsset_Subdomain(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets.*type.*subdomain").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(102)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type:   "subdomain",
		Domain: "sub.example.com",
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 102 {
		t.Errorf("id = %d, want 102", id)
	}
}

func TestInsertAsset_Endpoint(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets.*type.*endpoint").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(103)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type:   "endpoint",
		URL:    "https://example.com/api/v1",
		Method: "GET",
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 103 {
		t.Errorf("id = %d, want 103", id)
	}
}

func TestInsertAsset_App(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets.*type.*app").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(104)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type:     "app",
		BundleID: "com.example.app",
		AppName:  "Example App",
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 104 {
		t.Errorf("id = %d, want 104", id)
	}
}

func TestInsertAsset_UnknownType(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type: "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown asset type")
	}
}

func TestInsertAsset_OtherServiceMissing(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type: "service",
	})
	if err == nil {
		t.Fatal("expected error for service without url or service_name")
	}
}

func TestInsertAsset_OtherService(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(106)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type:        "service",
		ServiceName: "ssh",
		Port:        22,
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 106 {
		t.Errorf("id = %d, want 106", id)
	}
}

func TestInsertAsset_AppByName(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO assets").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(107)))
	mock.ExpectCommit()

	id, err := s.InsertAsset(ctx(), 1, tools.AssetInput{
		Type:    "app",
		AppName: "My App",
	})
	if err != nil {
		t.Fatalf("InsertAsset: %v", err)
	}
	if id != 107 {
		t.Errorf("id = %d, want 107", id)
	}
}

func TestListUntestedAssets(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)
	now := time.Now()

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery("SELECT .+ FROM assets a").
		WillReturnRows(sqlmock.NewRows(assetSelectColumns()).AddRow(
			int64(50), "root_domain", nil, "{1}",
			"untested.com", "untested.com", "", nil, nil,
			"", "{}",
			"", "{}",
			"", "", "", "", "",
			"", "", "",
			"", nil, nil, "",
			"{}", "", []byte("{}"),
			now, now, now,
		))

	assets, total, err := s.ListUntestedAssets(ctx(), 1, "", 50, 0)
	if err != nil {
		t.Fatalf("ListUntestedAssets: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(assets) != 1 {
		t.Fatalf("len(assets) = %d, want 1", len(assets))
	}
}

func TestListUntestedAssets_WithType(t *testing.T) {
	t.Parallel()
	s, mock := newRegexpMockStore(t)

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery("SELECT .+ FROM assets a").
		WillReturnRows(sqlmock.NewRows(assetSelectColumns()))

	assets, total, err := s.ListUntestedAssets(ctx(), 1, "service", 50, 0)
	if err != nil {
		t.Fatalf("ListUntestedAssets: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(assets) != 0 {
		t.Errorf("len(assets) = %d, want 0", len(assets))
	}
}
