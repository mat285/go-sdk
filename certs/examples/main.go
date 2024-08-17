package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/mat285/go-sdk/certs"
)

func main() {
	server, err := newServer()
	if err != nil {
		panic(err)
	}
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 8080})
	if err != nil {
		panic(err)
	}
	tl := tls.NewListener(l, server.TLSConfig)
	err = server.Serve(tl)
	if err != nil {
		panic(err)
	}
}

func newServer() (*http.Server, error) {
	config, err := getTLSConfig()
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
	}
	return server, nil
}

func getTLSConfig() (*tls.Config, error) {
	reload, err := certs.NewReloader(context.Background(), certs.OptReloaderDirs("./certs/examples/"))
	if err != nil {
		return nil, err
	}
	go reload.Start(context.Background())
	cfg := &tls.Config{}
	cfg.GetCertificate = reload.GetCertificate
	return cfg, nil
}
