package migration

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/blend/go-sdk/db"
)

type RunFunction func(context.Context, *db.Connection, *sql.Tx) error

type Migration struct {
	Revision    string
	Previous    string
	Description string
	Run         RunFunction
}

func New(opts ...Option) Migration {
	m := Migration{}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

func (m *Migration) Equal(o *Migration) bool {
	if m == nil && o == nil {
		return true
	}
	if m == nil || o == nil {
		return false
	}

	return m.Revision == o.Revision &&
		m.Previous == o.Previous &&
		m.Description == o.Description &&
		reflect.ValueOf(m.Run).Pointer() == reflect.ValueOf(o.Run).Pointer()
}

func RefMigration(m Migration) *Migration {
	return &m
}

func reverseSlice(ms []Migration) []Migration {
	ret := make([]Migration, len(ms))
	for i := 0; i < len(ms); i++ {
		ret[len(ms)-i-1] = ms[i]
	}
	return ret
}
