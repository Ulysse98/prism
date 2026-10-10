package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestPrismAPIRuntimeServerWaitsForActiveHandler(t *testing.T) {
	dir := t.TempDir()

	lock, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	started := make(chan struct{})
	finish := make(chan struct{})

	server := &http.Server{
		Handler: http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			close(started)
			<-finish
			_, _ = io.WriteString(writer, "ok")
		}),
	}
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type outcome struct {
		clean bool
		err   error
	}

	stopped := make(chan outcome, 1)

	go func() {
		clean, err := servePrismAPIUntilContext(
			ctx, server, listener,
		)
		stopped <- outcome{clean: clean, err: err}
	}()

	requested := make(chan error, 1)

	go func() {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		response, err := client.Get(
			"http://" + listener.Addr().String(),
		)
		if err != nil {
			requested <- err
			return
		}
		defer response.Body.Close()

		_, err = io.Copy(io.Discard, response.Body)
		requested <- err
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP handler did not start")
	}

	cancel()

	select {
	case result := <-stopped:
		t.Fatalf(
			"shutdown completed before handler: %+v",
			result,
		)
	case <-time.After(100 * time.Millisecond):
	}

	if other, err := acquirePrismAPIRuntimeLock(dir); err == nil {
		_ = other.Release()
		t.Fatal("runtime lock lost during HTTP request")
	}

	close(finish)

	select {
	case err := <-requested:
		if err != nil {
			t.Fatal("HTTP request failed:", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP request did not finish")
	}

	select {
	case result := <-stopped:
		if result.err != nil || !result.clean {
			t.Fatalf(
				"graceful shutdown failed: %+v",
				result,
			)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("graceful shutdown timed out")
	}

	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}

	restarted, err := acquirePrismAPIRuntimeLock(dir)
	if err != nil {
		t.Fatal("restart failed:", err)
	}

	if err := restarted.Release(); err != nil {
		t.Fatal(err)
	}
}
