package serve

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Server ties the webhook handler and the dispatcher to an http.Server and
// owns the shutdown sequence.
type Server struct {
	httpServer *http.Server
	handler    *Handler
	dispatcher *Dispatcher
	grace      time.Duration
	log        *slog.Logger
}

func NewServer(listen string, handler *Handler, dispatcher *Dispatcher, grace time.Duration, log *slog.Logger) *Server {
	mux := http.NewServeMux()
	mux.Handle("/webhooks/gitlab", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		queued, running := dispatcher.Stats()
		writeJSON(w, map[string]any{"status": "ok", "queued": queued, "running": running})
	})
	return &Server{
		httpServer: &http.Server{
			Addr:              listen,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		},
		handler:    handler,
		dispatcher: dispatcher,
		grace:      grace,
		log:        log,
	}
}

// Run serves until ctx is cancelled, then drains: stop accepting requests,
// let running reviews finish within the grace period, terminate stragglers.
func (s *Server) Run(ctx context.Context, workers int) error {
	// Workers run on a derived context so a listen failure (port in use, bad
	// address) can stop them too — Shutdown alone only waits for them.
	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	s.dispatcher.Start(workerCtx, workers)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()
	s.log.Info("webhook daemon listening", "addr", s.httpServer.Addr, "workers", workers)

	select {
	case err := <-errCh:
		// Listen failed outright; still stop workers and chat work before
		// returning.
		stopWorkers()
		s.dispatcher.Shutdown(0)
		s.handler.ShutdownChats(0)
		return err
	case <-ctx.Done():
	}

	s.log.Info("shutting down", "grace", s.grace.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.log.Warn("http shutdown", "error", err)
	}
	// Reviews and chats drain concurrently, sharing the grace period rather
	// than paying it twice.
	chatsDrained := make(chan struct{})
	go func() {
		s.handler.ShutdownChats(s.grace)
		close(chatsDrained)
	}()
	s.dispatcher.Shutdown(s.grace)
	<-chatsDrained
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
