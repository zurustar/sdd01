package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
)

func init() {
	sql.Register("sqlite", &Driver{})
}

// Driver is a minimal in-process driver that satisfies database/sql requirements
// without relying on CGO. It accepts any statements and reports success without
// mutating external state, allowing the persistence layer to exercise migration
// plumbing in pure Go environments.
type Driver struct{}

// Open returns a new connection backed by an in-memory stub implementation.
func (d *Driver) Open(name string) (driver.Conn, error) {
	return &conn{}, nil
}

type conn struct{}

type tx struct{}

type stmt struct{}

type rows struct{}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return &stmt{}, nil
}

func (c *conn) Close() error {
	return nil
}

func (c *conn) Begin() (driver.Tx, error) {
	return &tx{}, nil
}

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return &tx{}, nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return rows{}, nil
}

func (c *conn) CheckNamedValue(*driver.NamedValue) error {
	return nil
}

func (t *tx) Commit() error {
	return nil
}

func (t *tx) Rollback() error {
	return nil
}

func (s *stmt) Close() error {
	return nil
}

func (s *stmt) NumInput() int {
	return -1
}

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return rows{}, nil
}

func (r rows) Columns() []string {
	return nil
}

func (r rows) Close() error {
	return nil
}

func (r rows) Next(dest []driver.Value) error {
	return errors.New("sqlite stub driver: no rows")
}
