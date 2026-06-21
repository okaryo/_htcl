package http1

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
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
		{Name: "Connection", Value: "close"},
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
	if !<-closed {
		t.Fatal("client did not close the connection")
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
