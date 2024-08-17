package certs

import "time"

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
