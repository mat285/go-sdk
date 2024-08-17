package migration

import (
	"errors"
	"fmt"
)

// Sequence is a linked sequence of migrations
type Sequence struct {
	order map[string]Migration
	root  *Migration
	tail  *Migration
}

// NewSequence creates a new sequence from the migrations
func NewSequence(all []Migration) (*Sequence, error) {
	s := &Sequence{
		order: map[string]Migration{},
	}

	previous := map[string]bool{}

	for _, m := range all {

		if cur, has := s.order[m.Revision]; has {
			if !cur.Equal(&m) {
				return nil, fmt.Errorf("migrations with duplicate revisions: %v\n%v", cur, m)
			}
			continue
		} else {
			s.order[m.Revision] = m
		}
		if m.Previous != "" {
			previous[m.Previous] = true
			continue
		}
		if s.root != nil {
			return nil, fmt.Errorf("duplicate roots found %v %v", *s.root, m)
		}
		s.root = RefMigration(m)
	}

	if s.root == nil {
		return nil, errors.New("missing root migration")
	}

	for rev, m := range s.order {
		if previous[rev] {
			continue
		}
		if s.tail != nil {
			return nil, fmt.Errorf("duplicate tails found %v %v", *s.tail, m)
		}
		s.tail = RefMigration(m)
	}

	if s.tail == nil {
		return nil, errors.New("missing tail migration")
	}

	return s, nil
}

func (s *Sequence) Get(revision string) (*Migration, error) {
	if m, ok := s.order[revision]; ok {
		return &m, nil
	}
	return nil, errors.New("no migration found for revision")
}

func (s *Sequence) MigrationsFrom(start Migration) ([]Migration, error) {
	ret := []Migration{}

	count := 0
	curr := *s.tail

	for curr.Revision != start.Revision {
		ret = append(ret, curr)
		count++
		if curr.Previous == "" || count > len(s.order) {
			return nil, fmt.Errorf("migration %s not found in sequence", start.Revision)
		}
		curr = s.order[curr.Previous]
	}
	ret = append(ret, start)

	return reverseSlice(ret), nil
}

func (s *Sequence) All() ([]Migration, error) {
	return s.MigrationsFrom(*s.root)
}
