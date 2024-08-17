package certs

type ReloaderOption func(*Reloader)

func OptReloaderDirs(dirs ...string) ReloaderOption {
	return func(r *Reloader) {
		r.Dirs = dirs
	}
}
