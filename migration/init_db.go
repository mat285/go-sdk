package migration

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/blend/go-sdk/db"
)

const (
	statementTableExists = `
SELECT EXISTS (
  SELECT FROM 
    information_schema.tables 
  WHERE
    table_name = '%s'
)
`

	statementCreateTable = `
CREATE TABLE %s(
	id SERIAL,
	revision TEXT NOT NULL,
	previous TEXT
)
`
)

// CreateTableIfNotExists creates the migration table if it doesn't exist
func CreateTableIfNotExists(ctx context.Context, conn *db.Connection, txn *sql.Tx, table string) error {
	type dbBool struct {
		Exists bool `db:"exists"`
	}

	var exists dbBool
	statementExists := fmt.Sprintf(
		statementTableExists,
		table,
	)

	query := conn.Invoke(db.OptContext(ctx), db.OptTx(txn)).Query(statementExists)
	_, err := query.Out(&exists)
	if err != nil {
		return err
	}

	if exists.Exists {
		return nil // table was already created
	}

	statementCreate := fmt.Sprintf(
		statementCreateTable,
		table,
	)

	_, err = conn.Invoke(db.OptContext(ctx), db.OptTx(txn)).Exec(statementCreate)
	return err
}
