package database

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestScanRows_returnsRowsErr(t *testing.T) {
	errScanRowsTest := errors.New("scan rows test error")
	rows := &scanRowsTestRows{
		fields: []pgconn.FieldDescription{{Name: "value"}},
		data: [][]any{
			{"ok"},
		},
		errAfter: errScanRowsTest,
	}

	columns, err := Columns(rows)
	if err != nil {
		t.Fatalf("Columns() error = %v", err)
	}

	_, err = ScanRows(rows, len(columns))
	if !errors.Is(err, errScanRowsTest) {
		t.Fatalf("ScanRows() error = %v, want %v", err, errScanRowsTest)
	}
}

func Test_sqlValToString_nilTypedPointer(t *testing.T) {
	var s *string
	var iface interface{} = s

	got, err := sqlValToString(iface)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for nil *string, got %q", got)
	}
}

func Test_sqlValToString_nilInterface(t *testing.T) {
	got, err := sqlValToString(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
}

func Test_sqlValToString_validPointer(t *testing.T) {
	s := "hello"
	var iface interface{} = &s

	got, err := sqlValToString(iface)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

type scanRowsTestRows struct {
	fields   []pgconn.FieldDescription
	data     [][]any
	errAfter error
	curr     int
	closed   bool
}

func (r *scanRowsTestRows) Close() {
	r.closed = true
}

func (r *scanRowsTestRows) Err() error {
	if r.curr >= len(r.data) {
		return r.errAfter
	}
	return nil
}

func (r *scanRowsTestRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *scanRowsTestRows) FieldDescriptions() []pgconn.FieldDescription {
	return r.fields
}

func (r *scanRowsTestRows) Next() bool {
	if r.curr < len(r.data) {
		r.curr++
		return true
	}
	return false
}

func (r *scanRowsTestRows) Scan(dest ...any) error {
	return nil
}

func (r *scanRowsTestRows) Values() ([]any, error) {
	if r.curr > 0 && r.curr <= len(r.data) {
		return r.data[r.curr-1], nil
	}
	return nil, errors.New("no more rows")
}

func (r *scanRowsTestRows) RawValues() [][]byte {
	return nil
}

func (r *scanRowsTestRows) Conn() *pgx.Conn {
	return nil
}
