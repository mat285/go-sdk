package certs

import "time"

type ReloaderOption func(*Reloader)

func OptReloaderDirs(dirs ...string) ReloaderOption {
	return func(r *Reloader) {
		r.Dirs = dirs
	}
}

func OptReloaderInterval(inv time.Duration) ReloaderOption {
	return func(r *Reloader) {
		r.ReloadInterval = inv
	}
}

func OptReloaderWatch(watch bool) ReloaderOption {
	return func(r *Reloader) {
		r.Watch = watch
	}
}

func OptReloaderLogger(log Logger) ReloaderOption {
	return func(r *Reloader) {
		r.Log = log
	}
}
