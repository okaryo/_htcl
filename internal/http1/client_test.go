package http1

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestClientDoSendsRequestReadsResponseAndClosesConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	closed := make(chan bool, 1)
	go serveClientOnce(t, listener, requests, closed)

	request, err := NewRequest("GET", "/hello", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	client := Client{Timeout: 2 * time.Second}
	response, err := client.Do(listener.Addr().String(), request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}
	if got := string(response.Body); got != "hello" {
		t.Fatalf("Body = %q", got)
	}

	gotRequest := <-requests
	if !strings.HasPrefix(gotRequest, "GET /hello HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", gotRequest)
	}
	if !strings.Contains(gotRequest, "Connection: close\r\n") {
		t.Fatalf("missing Connection: close header:\n%s", gotRequest)
	}
	if !<-closed {
		t.Fatal("client did not close the connection")
	}
}

func TestClientDoStreamsRequestBodyFromReader(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveClientRequestWithBody(t, listener, requests)

	request, err := NewStreamingRequest("POST", "/upload", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, iotest.OneByteReader(strings.NewReader("hello")), 5)
	if err != nil {
		t.Fatalf("NewStreamingRequest: %v", err)
	}

	client := Client{Timeout: 2 * time.Second}
	response, err := client.Do(listener.Addr().String(), request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}

	gotRequest := <-requests
	if !strings.HasPrefix(gotRequest, "POST /upload HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", gotRequest)
	}
	if !strings.Contains(gotRequest, "Content-Length: 5\r\n") {
		t.Fatalf("missing Content-Length:\n%s", gotRequest)
	}
	if !strings.HasSuffix(gotRequest, "\r\n\r\nhello") {
		t.Fatalf("request body mismatch:\n%s", gotRequest)
	}
}

func TestClientDoTLSUsesTLSConnectionAndServerName(t *testing.T) {
	requests := make(chan string, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Method + " " + r.URL.RequestURI() + " " + r.Proto + "\r\nHost: " + r.Host + "\r\n"
		w.Header().Set("Content-Length", "5")
		w.Header().Set("Connection", "close")
		io.WriteString(w, "hello")
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	request, err := NewRequest("GET", "/hello", []HeaderField{
		{Name: "Host", Value: u.Host},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	client := Client{
		Timeout:   2 * time.Second,
		TLSConfig: &tls.Config{RootCAs: roots},
	}
	response, info, err := client.DoTLSWithInfo(u.Host, u.Hostname(), request)
	if err != nil {
		t.Fatalf("DoTLSWithInfo: %v", err)
	}

	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}
	if got := string(response.Body); got != "hello" {
		t.Fatalf("Body = %q", got)
	}
	if info.Version == "" {
		t.Fatal("TLS version was not captured")
	}
	if info.CipherSuite == "" {
		t.Fatal("TLS cipher suite was not captured")
	}
	if info.ServerName != u.Hostname() {
		t.Fatalf("TLS server name = %q, want %q", info.ServerName, u.Hostname())
	}
	if info.PeerCertificateCount == 0 {
		t.Fatal("peer certificate count was not captured")
	}
	if info.VerifiedChains == 0 {
		t.Fatal("verified certificate chain count was not captured")
	}
	gotRequest := <-requests
	if !strings.HasPrefix(gotRequest, "GET /hello HTTP/1.1\r\n") {
		t.Fatalf("request line mismatch:\n%s", gotRequest)
	}
	if !strings.Contains(gotRequest, "Host: "+u.Host+"\r\n") {
		t.Fatalf("Host header mismatch:\n%s", gotRequest)
	}
}

func TestClientDoTLSRejectsUntrustedCertificateBeforeRequest(t *testing.T) {
	requests := make(chan struct{}, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	request, err := NewRequest("GET", "/", []HeaderField{
		{Name: "Host", Value: u.Host},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	client := Client{Timeout: 2 * time.Second}
	_, err = client.DoTLS(u.Host, u.Hostname(), request)
	if err == nil {
		t.Fatal("expected an error")
	}
	var unknownAuthority x509.UnknownAuthorityError
	if !errors.As(err, &unknownAuthority) {
		t.Fatalf("expected unknown authority error, got %v", err)
	}
	select {
	case <-requests:
		t.Fatal("server received HTTP request after failed certificate verification")
	default:
	}
}

func TestClientDoTLSRejectsHostnameMismatchBeforeRequest(t *testing.T) {
	requests := make(chan struct{}, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	request, err := NewRequest("GET", "/", []HeaderField{
		{Name: "Host", Value: u.Host},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	client := Client{
		Timeout:   2 * time.Second,
		TLSConfig: &tls.Config{RootCAs: roots},
	}
	_, err = client.DoTLS(u.Host, "example.test", request)
	if err == nil {
		t.Fatal("expected an error")
	}
	var hostnameError x509.HostnameError
	if !errors.As(err, &hostnameError) {
		t.Fatalf("expected hostname error, got %v", err)
	}
	select {
	case <-requests:
		t.Fatal("server received HTTP request after failed hostname verification")
	default:
	}
}

func TestClientDoTLSSendsServerNameIndication(t *testing.T) {
	serverName := "example.test"
	cert, roots := newTestCertificate(t, serverName)
	sniValues := make(chan string, 1)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			sniValues <- info.ServerName
			return nil, nil
		},
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "5")
			w.Header().Set("Connection", "close")
			io.WriteString(w, "hello")
		}),
	}
	defer server.Close()
	go func() {
		if err := server.Serve(tls.NewListener(listener, tlsConfig)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve tls: %v", err)
		}
	}()

	request, err := NewRequest("GET", "/", []HeaderField{
		{Name: "Host", Value: serverName},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	client := Client{
		Timeout:   2 * time.Second,
		TLSConfig: &tls.Config{RootCAs: roots},
	}

	response, info, err := client.DoTLSWithInfo(listener.Addr().String(), serverName, request)
	if err != nil {
		t.Fatalf("DoTLSWithInfo: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}
	if got := <-sniValues; got != serverName {
		t.Fatalf("SNI = %q, want %q", got, serverName)
	}
	if info.ServerName != serverName {
		t.Fatalf("TLS info server name = %q, want %q", info.ServerName, serverName)
	}
}

func TestClientDoTLSNegotiatesHTTP11WithALPN(t *testing.T) {
	serverName := "example.test"
	cert, roots := newTestCertificate(t, serverName)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "5")
			w.Header().Set("Connection", "close")
			io.WriteString(w, "hello")
		}),
	}
	defer server.Close()
	go func() {
		if err := server.Serve(tls.NewListener(listener, tlsConfig)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve tls: %v", err)
		}
	}()

	request, err := NewRequest("GET", "/", []HeaderField{
		{Name: "Host", Value: serverName},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	client := Client{
		Timeout:   2 * time.Second,
		TLSConfig: &tls.Config{RootCAs: roots},
	}

	response, info, err := client.DoTLSWithInfo(listener.Addr().String(), serverName, request)
	if err != nil {
		t.Fatalf("DoTLSWithInfo: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", response.StatusCode)
	}
	if info.NegotiatedProtocol != "http/1.1" {
		t.Fatalf("negotiated protocol = %q, want http/1.1", info.NegotiatedProtocol)
	}
}

func TestClientDoAndDoTLSUseSameHTTPRequestShape(t *testing.T) {
	httpRequests := make(chan string, 1)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpRequests <- r.Method + " " + r.URL.RequestURI() + " " + r.Proto + "\r\nHost: " + r.Host + "\r\n"
		w.Header().Set("Content-Length", "5")
		w.Header().Set("Connection", "close")
		io.WriteString(w, "hello")
	}))
	defer httpServer.Close()

	httpsRequests := make(chan string, 1)
	httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpsRequests <- r.Method + " " + r.URL.RequestURI() + " " + r.Proto + "\r\nHost: " + r.Host + "\r\n"
		w.Header().Set("Content-Length", "5")
		w.Header().Set("Connection", "close")
		io.WriteString(w, "hello")
	}))
	defer httpsServer.Close()

	httpURL, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatalf("url.Parse http: %v", err)
	}
	httpsURL, err := url.Parse(httpsServer.URL)
	if err != nil {
		t.Fatalf("url.Parse https: %v", err)
	}
	httpRequest, err := NewRequest("GET", "/same?q=1", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest http: %v", err)
	}
	httpsRequest, err := NewRequest("GET", "/same?q=1", []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest https: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(httpsServer.Certificate())
	client := Client{
		Timeout:   2 * time.Second,
		TLSConfig: &tls.Config{RootCAs: roots},
	}
	if _, err := client.Do(httpURL.Host, httpRequest); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if _, err := client.DoTLS(httpsURL.Host, httpsURL.Hostname(), httpsRequest); err != nil {
		t.Fatalf("DoTLS: %v", err)
	}

	httpSeen := <-httpRequests
	httpsSeen := <-httpsRequests
	if httpSeen != httpsSeen {
		t.Fatalf("HTTP request shape differed:\nhttp:\n%s\nhttps:\n%s", httpSeen, httpsSeen)
	}
}

func TestClientDoOverridesConnectionKeepAliveForOneShotRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	closed := make(chan bool, 1)
	go serveClientOnce(t, listener, requests, closed)

	request, err := NewRequest("GET", "/hello", []HeaderField{
		{Name: "Host", Value: "example.test"},
		{Name: "Connection", Value: "keep-alive"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	client := Client{Timeout: 2 * time.Second}
	if _, err := client.Do(listener.Addr().String(), request); err != nil {
		t.Fatalf("Do: %v", err)
	}

	gotRequest := <-requests
	if strings.Contains(gotRequest, "Connection: keep-alive\r\n") {
		t.Fatalf("kept caller Connection header:\n%s", gotRequest)
	}
	if !strings.Contains(gotRequest, "Connection: close\r\n") {
		t.Fatalf("missing Connection: close header:\n%s", gotRequest)
	}
	if got := request.HeaderFields[1].Value; got != "keep-alive" {
		t.Fatalf("Client.Do mutated caller request header to %q", got)
	}
	<-closed
}

func TestConnectionRoundTripCanUseSameTCPConnectionTwice(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 2)
	go serveTwoRequestsOnOneConnection(t, listener, requests)

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	connection := NewConnection(conn, 2*time.Second)
	first := newTestRequest(t, "/first")
	second := newTestRequest(t, "/second")

	firstResponse, err := connection.RoundTrip(first)
	if err != nil {
		t.Fatalf("first RoundTrip: %v", err)
	}
	if got := string(firstResponse.Body); got != "one" {
		t.Fatalf("first body = %q", got)
	}
	if !connection.Reusable() {
		t.Fatal("connection should be reusable after first response")
	}

	secondResponse, err := connection.RoundTrip(second)
	if err != nil {
		t.Fatalf("second RoundTrip: %v", err)
	}
	if got := string(secondResponse.Body); got != "two" {
		t.Fatalf("second body = %q", got)
	}
	if !connection.Reusable() {
		t.Fatal("connection should be reusable after second response")
	}

	firstRequest := <-requests
	if !strings.HasPrefix(firstRequest, "GET /first HTTP/1.1\r\n") {
		t.Fatalf("first request mismatch:\n%s", firstRequest)
	}
	secondRequest := <-requests
	if !strings.HasPrefix(secondRequest, "GET /second HTTP/1.1\r\n") {
		t.Fatalf("second request mismatch:\n%s", secondRequest)
	}
}

func TestConnectionRoundTripMarksConnectionNotReusableForRequestClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveResponsesOnOneConnection(t, listener, requests, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello",
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	request, err := NewRequest("GET", "/close", []HeaderField{
		{Name: "Host", Value: "example.test"},
		{Name: "Connection", Value: "close"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	connection := NewConnection(conn, 2*time.Second)
	if _, err := connection.RoundTrip(request); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if connection.Reusable() {
		t.Fatal("connection should not be reusable after request Connection: close")
	}
	<-requests
}

func TestConnectionRoundTripMarksConnectionNotReusableForResponseClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveResponsesOnOneConnection(t, listener, requests, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello",
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	connection := NewConnection(conn, 2*time.Second)
	if _, err := connection.RoundTrip(newTestRequest(t, "/close")); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if connection.Reusable() {
		t.Fatal("connection should not be reusable after response Connection: close")
	}
	<-requests
}

func TestConnectionRoundTripMarksConnectionNotReusableAfterError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 1)
	go serveResponsesOnOneConnection(t, listener, requests, []string{
		"not an HTTP response\r\n\r\n",
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	connection := NewConnection(conn, 2*time.Second)
	if _, err := connection.RoundTrip(newTestRequest(t, "/bad")); err == nil {
		t.Fatal("expected an error")
	}
	if connection.Reusable() {
		t.Fatal("connection should not be reusable after response parse error")
	}
	<-requests
}

func TestClientDoReusableReusesIdleConnectionForSameAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	requests := make(chan string, 2)
	accepted := make(chan struct{}, 1)
	go serveTwoRequestsOnOneAcceptedConnection(t, listener, requests, accepted)

	client := &Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()

	firstResponse, err := client.DoReusable(listener.Addr().String(), newTestRequest(t, "/first"))
	if err != nil {
		t.Fatalf("first DoReusable: %v", err)
	}
	if got := string(firstResponse.Body); got != "one" {
		t.Fatalf("first body = %q", got)
	}

	secondResponse, err := client.DoReusable(listener.Addr().String(), newTestRequest(t, "/second"))
	if err != nil {
		t.Fatalf("second DoReusable: %v", err)
	}
	if got := string(secondResponse.Body); got != "two" {
		t.Fatalf("second body = %q", got)
	}

	firstRequest := <-requests
	if !strings.HasPrefix(firstRequest, "GET /first HTTP/1.1\r\n") {
		t.Fatalf("first request mismatch:\n%s", firstRequest)
	}
	secondRequest := <-requests
	if !strings.HasPrefix(secondRequest, "GET /second HTTP/1.1\r\n") {
		t.Fatalf("second request mismatch:\n%s", secondRequest)
	}

	select {
	case <-accepted:
	default:
		t.Fatal("server did not accept a connection")
	}
	select {
	case <-accepted:
		t.Fatal("client opened a second connection")
	default:
	}
}

func TestClientDoReusableDiscardsConnectionWhenResponseCloses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	accepted := make(chan struct{}, 2)
	go serveOneResponsePerAcceptedConnection(t, listener, accepted, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\nConnection: close\r\n\r\none",
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\ntwo",
	})

	client := &Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()

	firstResponse, err := client.DoReusable(listener.Addr().String(), newTestRequest(t, "/first"))
	if err != nil {
		t.Fatalf("first DoReusable: %v", err)
	}
	if got := string(firstResponse.Body); got != "one" {
		t.Fatalf("first body = %q", got)
	}

	secondResponse, err := client.DoReusable(listener.Addr().String(), newTestRequest(t, "/second"))
	if err != nil {
		t.Fatalf("second DoReusable: %v", err)
	}
	if got := string(secondResponse.Body); got != "two" {
		t.Fatalf("second body = %q", got)
	}

	<-accepted
	<-accepted
}

func TestClientDoReusableDiscardsExpiredIdleConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	accepted := make(chan struct{}, 2)
	go serveOneResponsePerAcceptedConnection(t, listener, accepted, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\none",
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\ntwo",
	})

	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	client := &Client{
		Timeout:     2 * time.Second,
		IdleTimeout: time.Second,
		now: func() time.Time {
			return now
		},
	}
	defer client.CloseIdleConnections()

	firstResponse, err := client.DoReusable(listener.Addr().String(), newTestRequest(t, "/first"))
	if err != nil {
		t.Fatalf("first DoReusable: %v", err)
	}
	if got := string(firstResponse.Body); got != "one" {
		t.Fatalf("first body = %q", got)
	}

	now = now.Add(2 * time.Second)
	secondResponse, err := client.DoReusable(listener.Addr().String(), newTestRequest(t, "/second"))
	if err != nil {
		t.Fatalf("second DoReusable: %v", err)
	}
	if got := string(secondResponse.Body); got != "two" {
		t.Fatalf("second body = %q", got)
	}

	<-accepted
	<-accepted
}

func TestClientDoReusableContextClosesConnectionOnCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	closed := make(chan bool, 1)
	go serveRequestThenWaitForClientClose(t, listener, closed)

	client := &Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = client.DoReusableContext(ctx, listener.Addr().String(), newTestRequest(t, "/cancel"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}

	select {
	case ok := <-closed:
		if !ok {
			t.Fatal("server did not observe client connection close")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for client connection close")
	}
}

func TestClientCloseIdleConnectionsReleasesIdleConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	closed := make(chan bool, 1)
	go serveReusableResponseThenWaitForClientClose(t, listener, closed)

	client := &Client{Timeout: 2 * time.Second}
	response, err := client.DoReusable(listener.Addr().String(), newTestRequest(t, "/idle"))
	if err != nil {
		t.Fatalf("DoReusable: %v", err)
	}
	if got := string(response.Body); got != "idle" {
		t.Fatalf("body = %q", got)
	}
	if got := client.idleConnectionCount(); got != 1 {
		t.Fatalf("idle connection count = %d, want 1", got)
	}

	client.CloseIdleConnections()

	if got := client.idleConnectionCount(); got != 0 {
		t.Fatalf("idle connection count = %d, want 0", got)
	}
	select {
	case ok := <-closed:
		if !ok {
			t.Fatal("server did not observe client connection close")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for idle connection close")
	}
}

func serveClientOnce(t *testing.T, listener net.Listener, requests chan<- string, closed chan<- bool) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	var request strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Errorf("read request line: %v", err)
			return
		}
		request.WriteString(line)
		if line == "\r\n" {
			break
		}
	}
	requests <- request.String()

	response := "HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello"
	if _, err := io.WriteString(conn, response); err != nil {
		t.Errorf("write response: %v", err)
		return
	}

	_, err = reader.ReadByte()
	closed <- err == io.EOF
}

func serveClientRequestWithBody(t *testing.T, listener net.Listener, requests chan<- string) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	var request strings.Builder
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Errorf("read request line: %v", err)
			return
		}
		request.WriteString(line)
		if name, value, ok := strings.Cut(strings.TrimRight(line, "\r\n"), ":"); ok && strings.EqualFold(name, "Content-Length") {
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				t.Errorf("parse Content-Length: %v", err)
				return
			}
		}
		if line == "\r\n" {
			break
		}
	}
	if contentLength > 0 {
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			t.Errorf("read request body: %v", err)
			return
		}
		request.Write(body)
	}
	requests <- request.String()

	response := "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok"
	if _, err := io.WriteString(conn, response); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func serveTwoRequestsOnOneConnection(t *testing.T, listener net.Listener, requests chan<- string) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	serveResponsesOnReader(t, conn, reader, requests, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\none",
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\ntwo",
	})
}

func serveTwoRequestsOnOneAcceptedConnection(t *testing.T, listener net.Listener, requests chan<- string, accepted chan<- struct{}) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	accepted <- struct{}{}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	serveResponsesOnReader(t, conn, reader, requests, []string{
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\none",
		"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\ntwo",
	})
}

func serveOneResponsePerAcceptedConnection(t *testing.T, listener net.Listener, accepted chan<- struct{}, responses []string) {
	t.Helper()

	for _, response := range responses {
		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		accepted <- struct{}{}

		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			conn.Close()
			t.Errorf("set deadline: %v", err)
			return
		}

		reader := bufio.NewReader(conn)
		if _, err := readHeaderBlock(reader); err != nil {
			conn.Close()
			t.Errorf("read request: %v", err)
			return
		}
		if _, err := io.WriteString(conn, response); err != nil {
			conn.Close()
			t.Errorf("write response: %v", err)
			return
		}
		conn.Close()
	}
}

func serveRequestThenWaitForClientClose(t *testing.T, listener net.Listener, closed chan<- bool) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	if _, err := readHeaderBlock(reader); err != nil {
		t.Errorf("read request: %v", err)
		return
	}

	_, err = reader.ReadByte()
	closed <- err == io.EOF
}

func serveReusableResponseThenWaitForClientClose(t *testing.T, listener net.Listener, closed chan<- bool) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	if _, err := readHeaderBlock(reader); err != nil {
		t.Errorf("read request: %v", err)
		return
	}
	if _, err := io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\nidle"); err != nil {
		t.Errorf("write response: %v", err)
		return
	}

	_, err = reader.ReadByte()
	closed <- err == io.EOF
}

func serveResponsesOnOneConnection(t *testing.T, listener net.Listener, requests chan<- string, responses []string) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Errorf("set deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)
	serveResponsesOnReader(t, conn, reader, requests, responses)
}

func serveResponsesOnReader(t *testing.T, conn net.Conn, reader *bufio.Reader, requests chan<- string, responses []string) {
	t.Helper()

	for _, response := range responses {
		request, err := readHeaderBlock(reader)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		requests <- request

		if _, err := io.WriteString(conn, response); err != nil {
			t.Errorf("write response: %v", err)
			return
		}
	}
}

func readHeaderBlock(reader *bufio.Reader) (string, error) {
	var request strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		request.WriteString(line)
		if line == "\r\n" {
			return request.String(), nil
		}
	}
}

func newTestRequest(t *testing.T, target string) *Request {
	t.Helper()

	request, err := NewRequest("GET", target, []HeaderField{
		{Name: "Host", Value: "example.test"},
	}, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return request
}

func newTestCertificate(t *testing.T, serverName string) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: serverName,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{serverName},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return cert, roots
}
