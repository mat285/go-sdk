package certs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type Cert struct {
	Name     string
	CertFile File
	KeyFile  File
	Loaded   time.Time
	tls.Certificate
}

func (c *Cert) DNSNames() []string {
	if c == nil {
		return nil
	}

	if len(c.Certificate.Certificate) == 0 {
		return nil
	}
	if c.Certificate.Leaf == nil {
		var err error
		c.Certificate.Leaf, err = x509.ParseCertificate(c.Certificate.Certificate[0])
		if err != nil {
			return nil
		}
	}
	return append(c.Certificate.Leaf.DNSNames, c.Certificate.Leaf.Subject.CommonName)
}

func (c *Cert) Reload() error {
	if c == nil {
		return fmt.Errorf("nil cert")
	}
	nc, err := LoadCertPair(c.Name, c.Loaded)
	if err != nil {
		return err
	}
	if nc == nil {
		return nil
	}
	c.Certificate = nc.Certificate
	c.Loaded = nc.Loaded
	return nil
}

func (c *Cert) File(ft FileType) *File {
	switch ft {
	case FileTypeCert:
		return &c.CertFile
	case FileTypeKey:
		return &c.KeyFile
	default:
		return nil
	}
}

func WildcardFor(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			return "*" + name[i:]
		}
	}
	return ""
}

func FilePairNameAndType(file string) (string, FileType) {
	ext := filepath.Ext(file)
	switch ext {
	case ".crt", ".key":
		name := file[:len(file)-len(ext)]
		t := FileType(ext[1:])
		return name, t
	default:
		return "", FileTypeUnknown
	}
}

func FilePairName(file string) string {
	name, _ := FilePairNameAndType(file)
	return name
}

func LoadCertPair(name string, mod time.Time) (*Cert, error) {
	certFile := name + ".crt"
	keyFile := name + ".key"

	cStat, err := os.Stat(certFile)
	if err != nil {
		return nil, err
	}
	kStat, err := os.Stat(keyFile)
	if err != nil {
		return nil, err
	}

	if mod.After(cStat.ModTime()) && mod.After(kStat.ModTime()) {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("no certs parsed")
	}
	xcert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}
	cert.Leaf = xcert

	now := time.Now()
	if now.After(cert.Leaf.NotAfter) || now.Before(cert.Leaf.NotBefore) {
		return nil, fmt.Errorf("invalid cert parsed")
	}

	return &Cert{
		Certificate: cert,
		Name:        name,
		CertFile: File{
			Path: certFile,
			Mod:  cStat.ModTime(),
		},
		KeyFile: File{
			Path: keyFile,
			Mod:  kStat.ModTime(),
		},
		Loaded: time.Now(),
	}, nil
}

func LoadDirectoryCerts(ctx context.Context, dir string) ([]*Cert, error) {
	files := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			return nil
		}
		name := FilePairName(path)
		if len(name) == 0 {
			return nil
		}
		files[name] = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	certs := make([]*Cert, 0, len(files))
	for name := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		cert, err := LoadCertPair(name, time.Time{})
		if err != nil || cert == nil {
			continue
		}
		certs = append(certs, cert)
	}
	return certs, nil
}
