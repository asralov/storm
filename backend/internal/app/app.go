// Package app owns the shared development and standalone server lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/asralov/storm/internal/api"
	"github.com/asralov/storm/internal/storage"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Options struct {
	Port         int
	DatabasePath string
	Wrap         func(http.Handler, string) http.Handler
	Ready        func(string)
}

func Run(ctx context.Context, opts Options) error {
	if opts.Port < 0 || opts.Port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.Port)))
	if err != nil {
		return fmt.Errorf("cannot listen on loopback port %d (another application may be using it): %w", opts.Port, err)
	}
	defer listener.Close()
	dbPath := opts.DatabasePath
	if dbPath == "" {
		dbPath, err = storage.DefaultPath()
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(dbPath) {
		return errors.New("database path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return err
	}
	// Never unlink this file: the OS releases the lock on exit, including crashes.
	lock, err := os.OpenFile(dbPath+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lockFile(lock); err != nil {
		return fmt.Errorf("Storm history is already in use by another instance: %w", err)
	}
	history, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer history.Close()
	recovery, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = history.RecoverInterrupted(recovery)
	cancel()
	if err != nil {
		return fmt.Errorf("recover history: %w", err)
	}
	server := api.NewServer(history)
	defer server.Close()
	var handler http.Handler = server
	if opts.Wrap != nil {
		handler = opts.Wrap(handler, listener.Addr().String())
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	serving := make(chan error, 1)
	go func() { serving <- httpServer.Serve(listener) }()
	address := "http://" + listener.Addr().String()
	log.Printf("Storm history: %s", dbPath)
	log.Printf("Storm API running at %s", address)
	if opts.Ready != nil {
		opts.Ready(address)
	}
	select {
	case <-ctx.Done():
	case err = <-serving:
	}
	// Join load workers and persist final results before closing SQLite.
	server.Close()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if stopErr := httpServer.Shutdown(shutdown); stopErr != nil {
		_ = httpServer.Close()
		if err == nil {
			err = stopErr
		}
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
