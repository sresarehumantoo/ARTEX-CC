package store

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func ctx() context.Context { return context.Background() }

func newMockStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &Store{DB: db}, mock
}

func newRegexpMockStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.ValueConverterOption(int64SliceConverter{}))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &Store{DB: db}, mock
}

// int64SliceConverter passes []int64 arguments through as pgx does, which
// the default converter rejects.
type int64SliceConverter struct{}

func (int64SliceConverter) ConvertValue(v any) (driver.Value, error) {
	if ids, ok := v.([]int64); ok {
		return ids, nil
	}
	return driver.DefaultParameterConverter.ConvertValue(v)
}
