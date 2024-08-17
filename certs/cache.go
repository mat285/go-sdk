package certs

import (
	"maps"
	"sync"
	"time"

	"github.com/blend/go-sdk/logger"
)

type Cache struct {
	lock     sync.Mutex
	log      Logger
	certs    map[string]*Cert
	sni      map[string]*Cert
	modified map[string]*Cert
}

func NewCache() *Cache {
	return &Cache{
		log:      logger.All(),
		certs:    make(map[string]*Cert),
		sni:      make(map[string]*Cert),
		modified: make(map[string]*Cert),
	}
}

func (c *Cache) Len() int {
	return len(c.certs)
}

func (c *Cache) GetSNI(dnsName string) *Cert {
	sni := c.sni
	if sni == nil {
		return nil
	}
	cert, has := sni[dnsName]
	if has {
		return cert
	}
	wildcard := WildcardFor(dnsName)
	if len(wildcard) == 0 {
		return nil
	}
	return sni[wildcard]
}

func (c *Cache) Get(name string) *Cert {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.certs[name]
}

func (c *Cache) SetModified(file string, mod time.Time) {
	name, ft := FilePairNameAndType(file)
	cert := c.Get(name)
	if cert == nil {
		return
	}
	f := cert.File(ft)
	if f == nil {
		return
	}
	f.Mod = mod
}

func (c *Cache) PopModified() []string {
	c.lock.Lock()
	m := c.modified
	c.modified = make(map[string]*Cert)
	c.lock.Unlock()
	ret := make([]string, 0, len(m))
	for name := range m {
		ret = append(ret, name)
	}
	return ret
}

func (c *Cache) Reload(name string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	var err error
	var added bool
	cert := c.certs[name]
	if cert != nil {
		added = true
		err = cert.Reload()
	} else {
		added = false
		cert, err = LoadCertPair(name, time.Time{})
	}
	if err != nil {
		return false, err
	}
	c.set(cert)
	return added, nil
}

func (c *Cache) Set(certs ...*Cert) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.set(certs...)
}

func (c *Cache) set(certs ...*Cert) {
	for _, cert := range certs {
		if cert == nil {
			continue
		}
		dnsNames := cert.DNSNames()
		logger.MaybeDebugf(c.log, "Setting cert name to cache %s, DNSNames: %v", cert.Name, dnsNames)
		c.certs[cert.Name] = cert

		// copy map to allow readers to keep accessing without lock
		sni := make(map[string]*Cert, len(c.sni))
		maps.Copy(sni, c.sni)
		for _, dn := range dnsNames {
			sni[dn] = cert
		}
		c.sni = sni
	}
}
