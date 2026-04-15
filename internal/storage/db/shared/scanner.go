package shared

// RowScanner abstracts *sql.Row and *sql.Rows for shared scan helpers in repositories.
type RowScanner interface {
	Scan(dest ...interface{}) error
}
