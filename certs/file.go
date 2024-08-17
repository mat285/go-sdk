package certs

import (
	"path/filepath"
	"sort"
	"time"
)

type FileType string

const (
	FileTypeUnknown FileType = ""
	FileTypeCert    FileType = "crt"
	FileTypeKey     FileType = "key"
)

type File struct {
	Path string
	Mod  time.Time
}

func RemoveSubdirectories(dirs []string) ([]string, error) {
	for i := 0; i < len(dirs); i++ {
		if len(dirs[i]) == 0 {
			continue
		}
		a, err := filepath.Abs(dirs[i])
		if err != nil {
			return nil, err
		}
		dirs[i] = a
	}
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) < len(dirs[j])
	})

	set := make(map[string]bool)

	for i := 0; i < len(dirs); i++ {
		dir := dirs[i]
		for len(dir) > 0 && dir[len(dir)-1] == '/' {
			dir = dir[:len(dir)-1]
		}
		if len(dirs[i]) == 0 {
			continue
		}
		if isSubdirOf(set, dirs[i]) {
			continue
		}
		set[dir] = true
	}
	ret := make([]string, 0, len(set))
	for d := range set {
		ret = append(ret, d)
	}
	return ret, nil
}

func isSubdirOf(set map[string]bool, dir string) bool {
	if set[dir] {
		return true
	}

	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' {
			if set[dir[:i]] {
				return true
			}
		}
	}
	return false
}
