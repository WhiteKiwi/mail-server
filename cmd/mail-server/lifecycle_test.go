package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownWaitsForInflightRequest(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { close(entered); <-release; w.WriteHeader(204) })}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	finished := make(chan error, 1)
	go func() { finished <- serve(ctx, s, l, time.Second) }()
	response := make(chan error, 1)
	go func() {
		r, err := http.Get("http://" + l.Addr().String())
		if err == nil {
			_, _ = io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}
		response <- err
	}()
	<-entered
	stop()
	select {
	case err := <-finished:
		t.Fatalf("returned before request finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
