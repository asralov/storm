package loadtest

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestFailureClassification(t *testing.T) {
	for _, body := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[body], func(t *testing.T) {
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if body {
					w.Header().Set("Content-Length", "100")
					w.Write([]byte("partial"))
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer target.Close()
			client := target.Client()
			client.Timeout = 100 * time.Millisecond
			var s Summary
			s.Record(executeRequest(context.Background(), client, target.URL))
			if s.NetworkErrors != 1 || s.Canceled != 0 || s.Successes != 0 {
				t.Fatalf("client timeout: %+v", s)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			s = Summary{}
			s.Record(executeRequest(ctx, client, target.URL))
			if s.Canceled != 1 || s.NetworkErrors != 0 {
				t.Fatalf("run cancellation: %+v", s)
			}
		})
	}
}

func TestTruncatedBodyIsTransportFailure(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.Write([]byte("short"))
	}))
	defer target.Close()
	var s Summary
	s.Record(executeRequest(context.Background(), target.Client(), target.URL))
	if s.Successes != 0 || s.NetworkErrors != 1 || s.HTTPErrorCount != 0 {
		t.Fatalf("partial 200: %+v", s)
	}
}

func TestLocalhostCaseAndConnectionBoundary(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := strings.Split(listener.Addr().String(), ":")[1]
	for _, host := range []string{"localhost", "LOCALHOST", "LocalHost"} {
		conn, err := dialLoopback(context.Background(), "tcp", net.JoinHostPort(host, port))
		if err != nil {
			t.Fatal(host, err)
		}
		conn.Close()
	}
	for _, host := range []string{"example.com", "localhost.example.com", "192.0.2.1", "0.0.0.0", "::"} {
		conn, err := dialLoopback(context.Background(), "tcp", net.JoinHostPort(host, port))
		if err == nil {
			conn.Close()
			t.Fatal("nonloopback accepted", host)
		}
	}
}
