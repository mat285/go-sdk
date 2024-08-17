package certs

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/blend/go-sdk/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/mat285/go-sdk/sync/collections"
)

var (
	ErrAlreadyRunning = errors.New("already running")
)

type Reloader struct {
	Lock           sync.Mutex
	Log            Logger
	Dirs           []string
	ReloadInterval time.Duration
	Watch          bool

	watcher *fsnotify.Watcher

	running     bool
	certs       *Cache
	reloadQueue *collections.Set[string]
	stopped     chan struct{}
	runCtx      context.Context
	runCancel   context.CancelFunc
}

type Logger interface {
	logger.OutputReceiver
	logger.ErrorOutputReceiver
}

func NewReloader(ctx context.Context, opts ...ReloaderOption) (*Reloader, error) {
	r := &Reloader{
		reloadQueue: collections.NewSet[string](32),
	}
	for _, opt := range opts {
		opt(r)
	}

	sanitized, err := RemoveSubdirectories(r.Dirs)
	if err != nil {
		return nil, err
	}
	r.Dirs = sanitized
	return r, r.initialize(ctx)
}

func (r *Reloader) Start(ctx context.Context) error {
	return r.run(ctx)
}

func (r *Reloader) Stop() error {
	if !r.running {
		return nil
	}
	r.Lock.Lock()
	stop := r.stopped
	cancel := r.runCancel
	r.Lock.Unlock()
	cancel()
	<-stop
	return nil
}

func (r *Reloader) GetCertificate(helo *tls.ClientHelloInfo) (*tls.Certificate, error) {
	server := helo.ServerName
	cert := r.certs.GetSNI(server)
	if cert == nil {
		return nil, fmt.Errorf("no cert for name %s", server)
	}
	return &cert.Certificate, nil
}

func (r *Reloader) run(ctx context.Context) error {
	if r.running {
		return ErrAlreadyRunning
	}
	r.Lock.Lock()
	if r.running {
		r.Lock.Unlock()
		return ErrAlreadyRunning
	}
	logger.MaybeInfofContext(ctx, r.Log, "Running cert reloader with directories %s", r.Dirs)
	r.running = true
	r.runCtx, r.runCancel = context.WithCancel(ctx)
	r.stopped = make(chan struct{})
	errs := make(chan error, 2)
	stop := r.stopped
	cancel := r.runCancel
	defer cancel()
	go func() { errs <- r.watch(r.runCtx) }()
	go func() { errs <- r.processQueue(r.runCtx) }()
	r.Lock.Unlock()

	var err1, err2 error
	select {
	case <-ctx.Done():
		logger.MaybeInfo(r.Log, "Stopping reloader")
		cancel()
		err1 = <-errs
	case err1 = <-errs:
		logger.MaybeInfo(r.Log, "Stopping reloader")
		cancel()
	}

	err2 = <-errs
	close(errs)
	close(stop)
	logger.MaybeInfo(r.Log, "Reloader stopped")
	return errors.Join(err1, err2)
}

func (r *Reloader) Initialize(ctx context.Context) error {
	r.Lock.Lock()
	defer r.Lock.Unlock()
	return r.initialize(ctx)
}

func (r *Reloader) initialize(ctx context.Context) error {
	err := r.initializeAllCerts(ctx)
	if err != nil {
		return err
	}

	if r.Watch {
		err := r.initializeWatch()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reloader) initializeWatch() error {
	var err error
	if r.watcher == nil {
		r.watcher, err = fsnotify.NewWatcher()
		if err != nil {
			return err
		}
	}

	for _, dir := range r.Dirs {
		err = r.watcher.Add(dir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reloader) loadAllCerts(ctx context.Context) error {
	errs := make([]error, 0, len(r.Dirs))
	for _, dir := range r.Dirs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		logger.MaybeDebugfContext(ctx, r.Log, "Loading certs for directory %s", dir)
		certs, err := LoadDirectoryCerts(ctx, dir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		r.certs.Set(certs...)
	}
	return errors.Join(errs...)
}

func (r *Reloader) initializeAllCerts(ctx context.Context) error {
	if r.certs == nil {
		r.certs = NewCache(r.Log)
	}
	err := r.loadAllCerts(ctx)
	if err != nil {
		return err
	}
	r.reloadQueue = collections.NewSet[string](r.certs.Len() * 3)
	return nil
}

func (r *Reloader) processQueue(ctx context.Context) error {
	for {
		val, err := r.reloadQueue.Poll(ctx)
		if err != nil {
			return err
		}
		if val == nil {
			return nil
		}
		name := *val
		logger.MaybeDebugfContext(ctx, r.Log, "Processing cert reload %s", name)
		add, err := r.certs.Reload(name)
		if err != nil {
			logger.MaybeErrorfContext(ctx, r.Log, "Error reloading cert pair %s: %v", name, err)
			continue
		}
		if add {
			if r.certs.Len() >= (2*r.reloadQueue.Cap())/3 {
				logger.MaybeDebugfContext(ctx, r.Log, "Resizing queue for new certs")
				r.reloadQueue.Expand(2 * r.reloadQueue.Cap())
			}
		}
		continue
	}
}

func (r *Reloader) watch(ctx context.Context) error {
	if r.ReloadInterval <= 0 && r.watcher == nil {
		return fmt.Errorf("cannot reload certs when both interval and watch are disabled")
	}
	var tick <-chan time.Time
	if r.ReloadInterval > 0 {
		logger.MaybeInfofContext(ctx, r.Log, "Using reload interval %0.2f seconds", float64(r.ReloadInterval)/float64(time.Second))
		ticker := time.NewTicker(r.ReloadInterval)
		defer ticker.Stop()
		tick = ticker.C
	} else {
		logger.MaybeInfoContext(ctx, r.Log, "Reload on interval disabled. This is NOT reccomended")
		t := make(chan time.Time)
		tick = t
		defer close(t)
	}

	var fsevents chan fsnotify.Event
	var fserrs chan error
	if r.watcher != nil {
		logger.MaybeInfoContext(ctx, r.Log, "FS Watch is configured")
		fsevents = r.watcher.Events
		fserrs = r.watcher.Errors
		defer func() {
			r.watcher.Close()
		}()
	} else {
		logger.MaybeInfoContext(ctx, r.Log, "FS Watch is not configured")
		fsevents = make(chan fsnotify.Event)
		fserrs = make(chan error)
		defer func() {
			close(fsevents)
			close(fserrs)
		}()
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-tick:
			if !ok {
				return nil
			}
			logger.MaybeDebugfContext(ctx, r.Log, "Reloading all certs")
			r.reloadQueue.Empty()
			err := r.loadAllCerts(ctx)
			if err != nil {
				logger.MaybeErrorfContext(ctx, r.Log, "Reload all error: %v", err)
			}
			if r.certs.Len() >= (2*r.reloadQueue.Cap())/3 {
				logger.MaybeDebugfContext(ctx, r.Log, "Resizing queue for new certs len: %d cap: %d", r.certs.Len(), r.reloadQueue.Cap())
				r.reloadQueue.Expand(2 * r.reloadQueue.Cap())
			}
			continue
		case event, ok := <-fsevents:
			if !ok {
				return nil
			}
			r.handleEvent(ctx, event)
			continue
		case werr, ok := <-fserrs:
			if !ok {
				return nil
			}
			logger.MaybeErrorfContext(ctx, r.Log, "File watcher error: %v", werr)
			continue
		}
	}
}

func (r *Reloader) handleEvent(ctx context.Context, event fsnotify.Event) {
	if r.reloadQueue == nil {
		return
	}
	switch event.Op {
	case fsnotify.Create, fsnotify.Write:
		name := FilePairName(event.Name)
		if len(name) == 0 {
			return
		}
		logger.MaybeDebugfContext(ctx, r.Log, "Got write event for name %s pushing to write update", name)
		r.reloadQueue.Push(name)
		return
	default:
		return
	}
}
