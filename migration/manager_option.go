package migration

type ManagerOption func(*Manager) error

// OptManagerTable sets the table name on a manager.
func OptManagerTable(table string) ManagerOption {
	return func(m *Manager) error {
		m.Table = table
		return nil
	}
}

// OptManagerSequence sets the migrations on a manager.
func OptManagerSequence(migrations *Sequence) ManagerOption {
	return func(m *Manager) error {
		m.Migrations = migrations
		return nil
	}
}
