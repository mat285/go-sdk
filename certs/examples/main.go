package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/blend/go-sdk/logger"
	"github.com/mat285/go-sdk/certs"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	server, err := newServer(ctx)
	if err != nil {
		panic(err)
	}
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 8080})
	if err != nil {
		panic(err)
	}
	tl := tls.NewListener(l, server.TLSConfig)

	go func() {
		<-ctx.Done()
		logger.All().Info("Shutting down server")
		server.Shutdown(ctx)

	}()

	err = server.Serve(tl)
	cancel()
	fmt.Println(err)
}

func newServer(ctx context.Context) (*http.Server, error) {
	config, err := getTLSConfig(ctx)
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("hello world"))
		}),
		TLSConfig: config,
		// BaseContext: func(_ net.Listener) context.Context { return ctx },
	}
	return server, nil
}

func getTLSConfig(ctx context.Context) (*tls.Config, error) {
	reload, err := certs.NewReloader(
		context.Background(),
		certs.OptReloaderDirs("./certs/examples/", "certs/"),
		certs.OptReloaderWatch(true),
		certs.OptReloaderInterval(5*time.Second),
		certs.OptReloaderLogger(logger.All()),
	)
	if err != nil {
		return nil, err
	}
	go reload.Start(ctx)
	cfg := &tls.Config{}
	cfg.GetCertificate = reload.GetCertificate
	return cfg, nil
}
