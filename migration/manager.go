package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/blend/go-sdk/db"
)

const (
	// DefaultMetadataTable is the default name for the table
	DefaultMetadataTable = "migrations"
)

const (
	migrationInsertStatementFmt = `
INSERT INTO %s (revision, previous)
VALUES ('%s', '%s')
`

	migrationInsertStatementNoPreviousFmt = `
INSERT INTO %s (revision, previous)
VALUES ('%s', NULL)
`

	selectLatestRevisionStatementFmt = `
SELECT revision FROM %s ORDER BY id desc LIMIT 1
`
)

type Manager struct {
	Table      string
	Schema     string
	Migrations *Sequence
}

func NewManager(opts ...ManagerOption) (*Manager, error) {
	m := &Manager{Table: DefaultMetadataTable}
	for _, opt := range opts {
		err := opt(m)
		if err != nil {
			return nil, err
		}
	}

	return m, nil
}

// Apply applies the migrations
func (m *Manager) Apply(ctx context.Context, conn *db.Connection) (err error) {
	var tx *sql.Tx
	tx, err = conn.Begin()
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			if txErr := tx.Rollback(); txErr != nil {
				err = errors.Join(err, txErr)
			}
		} else {
			if txErr := tx.Commit(); txErr != nil {
				err = errors.Join(err, txErr)
			}
		}
	}()

	err = m.PrepareDB(ctx, conn, tx)
	if err != nil {
		return
	}

	err = m.applyInternal(ctx, conn, tx)
	return
}

// PrepareDB runs the steps necessary to prepare a db for this migration manager
func (m *Manager) PrepareDB(ctx context.Context, conn *db.Connection, tx *sql.Tx) error {
	return CreateTableIfNotExists(ctx, conn, tx, m.Table)
}

func (m *Manager) applyInternal(ctx context.Context, conn *db.Connection, tx *sql.Tx) error {
	migration, err := m.StartingMigration(ctx, conn, tx)
	if err != nil {
		return err
	}

	start := 0
	seq, err := m.Migrations.All()
	if err != nil {
		return err
	}

	if migration != nil {
		seq, err = m.Migrations.MigrationsFrom(*migration)
		if err != nil {
			return err
		}
		start = 1
	}

	// skip the start, already in DB
	for i := start; i < len(seq); i++ {
		err = m.ApplyMigration(ctx, conn, tx, seq[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) StartingMigration(ctx context.Context, conn *db.Connection, tx *sql.Tx) (*Migration, error) {
	type revisionStr struct {
		Revision string `db:"revision"`
	}
	var revision revisionStr
	statement := fmt.Sprintf(selectLatestRevisionStatementFmt, m.Table)
	query := conn.Invoke(db.OptContext(ctx), db.OptTx(tx)).Query(statement)
	_, err := query.Out(&revision)
	if err != nil {
		return nil, err
	}

	if revision.Revision == "" {
		return nil, nil
	}

	return m.Migrations.Get(revision.Revision)
}

func (m *Manager) ApplyMigration(ctx context.Context, conn *db.Connection, tx *sql.Tx, migration Migration) error {
	if err := migration.Run(ctx, conn, tx); err != nil {
		return err
	}

	return m.InsertMigration(ctx, conn, tx, migration)
}

func (m *Manager) InsertMigration(ctx context.Context, conn *db.Connection, tx *sql.Tx, migration Migration) error {
	statement := fmt.Sprintf(
		migrationInsertStatementNoPreviousFmt,
		m.Table,
		migration.Revision,
	)
	if migration.Previous != "" {
		statement = fmt.Sprintf(
			migrationInsertStatementFmt,
			m.Table,
			migration.Revision,
			migration.Previous,
		)
	}

	_, err := conn.Invoke(db.OptContext(ctx), db.OptTx(tx)).Exec(statement)
	return err
}
