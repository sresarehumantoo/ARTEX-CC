package store

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func insertRawAsset(t *testing.T, s *Store, typ, serviceType, method, url string, taskIDs []int64) int64 {
	t.Helper()
	var id int64
	err := s.QueryRowContext(context.Background(), `
		INSERT INTO assets (type, service_type, method, url, task_ids)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''), $4, $5::bigint[]) RETURNING id`,
		typ, serviceType, method, url, taskIDs).Scan(&id)
	if err != nil {
		t.Fatalf("insert asset %q: %v", url, err)
	}
	return id
}

func assetURL(t *testing.T, s *Store, id int64) string {
	t.Helper()
	var u string
	if err := s.QueryRowContext(context.Background(), `SELECT url FROM assets WHERE id = $1`, id).Scan(&u); err != nil {
		t.Fatal(err)
	}
	return u
}

func rerunAssetURLBackfill(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.ExecContext(ctx, backfillsTable); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, `DELETE FROM store_backfills WHERE name = $1`, assetURLBackfill); err != nil {
		t.Fatal(err)
	}
	if err := s.BackfillAssetURLs(ctx); err != nil {
		t.Fatalf("BackfillAssetURLs: %v", err)
	}
}

func TestBackfillAssetURLs(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	taskID, _ := testEngagement(t, s, "asset-url-backfill")
	otherTask, _ := testEngagement(t, s, "asset-url-backfill-other")
	h := fmt.Sprintf("bf%d", time.Now().UnixNano())

	withCreds := insertRawAsset(t, s, AssetService, "http", "",
		"HTTP://admin:s%40cret@Box."+h+".Example.com:80/Login?x=1", []int64{taskID, otherTask})
	insertRawAsset(t, s, AssetService, "http", "", "http://dup."+h+".example.com/", []int64{taskID})
	caseDup := insertRawAsset(t, s, AssetService, "http", "", "http://u:p@DUP."+h+".example.com/", []int64{taskID})
	insertRawAsset(t, s, AssetService, "http", "", "http://same."+h+".example.com/", []int64{taskID})
	fullDup := insertRawAsset(t, s, AssetService, "http", "", "http://x:y@same."+h+".example.com/", []int64{taskID})
	endpoint := insertRawAsset(t, s, AssetEndpoint, "", "GET",
		"https://svc:pw@EP."+h+".example.com:443/api", []int64{taskID, 1 << 40})

	rerunAssetURLBackfill(t, s)

	checks := []struct {
		id   int64
		want string
	}{
		{withCreds, "http://box." + h + ".example.com/Login?x=1"},
		{caseDup, "http://DUP." + h + ".example.com/"},
		{fullDup, "http://x:y@same." + h + ".example.com/"},
		{endpoint, "https://ep." + h + ".example.com/api"},
	}
	for _, c := range checks {
		if got := assetURL(t, s, c.id); got != c.want {
			t.Errorf("asset %d url = %q, want %q", c.id, got, c.want)
		}
	}

	creds := map[string]rawCredential{}
	for _, c := range taskCredentials(t, s, taskID) {
		creds[c.Domain+"/"+c.Username] = c
	}
	wantCreds := map[string]string{
		"box." + h + ".example.com/admin": "s@cret",
		"dup." + h + ".example.com/u":     "p",
		"same." + h + ".example.com/x":    "y",
		"ep." + h + ".example.com/svc":    "pw",
	}
	if len(creds) != len(wantCreds) {
		t.Errorf("got %d credentials, want %d: %+v", len(creds), len(wantCreds), creds)
	}
	for k, pw := range wantCreds {
		if c, ok := creds[k]; !ok || c.Password != pw || c.Source != "asset_url" {
			t.Errorf("credential %s = %+v, want password %q from asset_url", k, c, pw)
		}
	}
	if other := taskCredentials(t, s, otherTask); len(other) != 1 || other[0].Domain != "box."+h+".example.com" {
		t.Errorf("other task credentials = %+v, want the box credential", other)
	}

	if err := s.BackfillAssetURLs(ctx); err != nil {
		t.Fatal(err)
	}
	before := taskCredentials(t, s, taskID)
	rerunAssetURLBackfill(t, s)
	for _, c := range checks {
		if got := assetURL(t, s, c.id); got != c.want {
			t.Errorf("second run: asset %d url = %q, want %q", c.id, got, c.want)
		}
	}
	if after := taskCredentials(t, s, taskID); !reflect.DeepEqual(before, after) {
		t.Errorf("second run changed credentials:\nbefore %+v\nafter  %+v", before, after)
	}
}
