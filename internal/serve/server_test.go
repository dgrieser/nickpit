package serve

import (
	"context"
	"net"
	"testing"
	"time"
)

// A listen failure must stop the workers and return the bind error instead
// of hanging in Shutdown.
func TestServerRunReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	handler, dispatcher, _ := newHandlerEnv(t)
	server := NewServer(listener.Addr().String(), handler, dispatcher, time.Second, discardLogger())

	done := make(chan error, 1)
	go func() { done <- server.Run(context.Background(), 2) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected bind error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run hung after listen failure")
	}
}
