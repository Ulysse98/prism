package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const prismAPIShutdownTimeout = 10 * time.Second

// clean is true only when releasing the API runtime lock
// is safe.
func servePrismAPIWithSignals(
	server *http.Server,
) (bool, error) {
	if server == nil {
		return false, fmt.Errorf("nil HTTP server")
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		// No listener started serving HTTP requests.
		return true, err
	}

	return servePrismAPIUntilContext(
		ctx, server, listener,
	)
}

// A clean result requires a successful graceful Shutdown.
// Unexpected Serve exits do not certify handler completion.
func servePrismAPIUntilContext(
	ctx context.Context,
	server *http.Server,
	listener net.Listener,
) (bool, error) {
	if ctx == nil || server == nil || listener == nil {
		return false, fmt.Errorf("invalid HTTP lifecycle inputs")
	}

	served := make(chan error, 1)

	go func() {
		served <- server.Serve(listener)
	}()

	select {
	case err := <-served:
		_ = server.Close()

		return false, fmt.Errorf(
			"HTTP server exited without graceful shutdown: %v",
			err,
		)

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			prismAPIShutdownTimeout,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-served

			// A forced Close does not prove that active
			// handlers completed. Retain the runtime lock.
			return false, fmt.Errorf(
				"HTTP graceful shutdown incomplete: %w",
				err,
			)
		}

		err := <-served

		if !errors.Is(err, http.ErrServerClosed) {
			return false, fmt.Errorf(
				"unexpected Serve result after shutdown: %v",
				err,
			)
		}

		return true, nil
	}
}
