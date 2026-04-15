package db

// rowScanner abstracts *sql.Row and *sql.Rows for shared scan helpers in auth repositories.
type rowScanner interface {
	Scan(dest ...interface{}) error
}
