package migration

type Option func(*Migration)

// OptPrevious sets the previous on a migration.
func OptPrevious(previous string) Option {
	return func(m *Migration) {
		m.Previous = previous
	}
}

// OptRevision sets the revision on a migration.
func OptRevision(revision string) Option {
	return func(m *Migration) {
		m.Revision = revision
	}
}

// OptDescription sets the description on a migration.
func OptDescription(description string) Option {
	return func(m *Migration) {
		m.Description = description

	}
}

// OptRun sets the `run` function on a migration.
func OptRun(run RunFunction) Option {
	return func(m *Migration) {
		m.Run = run
	}
}
