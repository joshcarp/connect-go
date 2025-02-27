// Copyright 2021-2023 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connect_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
)

var examplePingServer *inMemoryServer

func init() {
	// Generally, init functions are bad.
	//
	// To write testable examples that users can grok *and* can execute in the
	// playground, where networking is disabled, we need an HTTP server that uses
	// in-memory pipes instead of TCP. We don't want to pollute every example
	// with this setup code.
	//
	// The least-awful option is to set up the server in init().
	mux := http.NewServeMux()
	mux.Handle(pingv1connect_test.NewPingServiceHandler(pingServer{}))
	examplePingServer = newInMemoryServer(mux)
}

// inMemoryServer is an HTTP server that uses in-memory pipes instead of TCP.
// It supports HTTP/2 and has TLS enabled.
//
// The Go Playground panics if we try to start a TCP-backed server. If you're
// not familiar with the Playground's behavior, it looks like our examples are
// broken. This server lets us write examples that work in the playground
// without abstracting over HTTP.
type inMemoryServer struct {
	server   *httptest.Server
	listener *memoryListener
}

// newInMemoryServer constructs and starts an inMemoryServer.
func newInMemoryServer(handler http.Handler) *inMemoryServer {
	lis := &memoryListener{
		conns:  make(chan net.Conn),
		closed: make(chan struct{}),
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = lis
	server.EnableHTTP2 = true
	server.StartTLS()
	return &inMemoryServer{
		server:   server,
		listener: lis,
	}
}

// Client returns an HTTP client configured to trust the server's TLS
// certificate and use HTTP/2 over an in-memory pipe. Automatic HTTP-level gzip
// compression is disabled. It closes its idle connections when the server is
// closed.
func (s *inMemoryServer) Client() *http.Client {
	client := s.server.Client()
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.DialContext = s.listener.DialContext
		transport.DisableCompression = true
	}
	return client
}

// URL is the server's URL.
func (s *inMemoryServer) URL() string {
	return s.server.URL
}

// Close shuts down the server, blocking until all outstanding requests have
// completed.
func (s *inMemoryServer) Close() {
	s.server.Close()
}

type memoryListener struct {
	conns  chan net.Conn
	once   sync.Once
	closed chan struct{}
}

// Accept implements net.Listener.
func (l *memoryListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.closed:
		return nil, errors.New("listener closed")
	}
}

// Close implements net.Listener.
func (l *memoryListener) Close() error {
	l.once.Do(func() {
		close(l.closed)
	})
	return nil
}

// Addr implements net.Listener.
func (l *memoryListener) Addr() net.Addr {
	return &memoryAddr{}
}

// DialContext is the type expected by http.Transport.DialContext.
func (l *memoryListener) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	select {
	case <-l.closed:
		return nil, errors.New("listener closed")
	default:
	}
	server, client := net.Pipe()
	l.conns <- server
	return client, nil
}

type memoryAddr struct{}

// Network implements net.Addr.
func (*memoryAddr) Network() string { return "memory" }

// String implements io.Stringer, returning a value that matches the
// certificates used by net/http/httptest.
func (*memoryAddr) String() string { return "example.com" }
