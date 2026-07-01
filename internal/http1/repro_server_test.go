package http1

import (
	"bufio"
	"io"
	"net"
	"testing"
	"time"
)

type reproServer struct {
	address  string
	requests chan string
}

func startSilentResponseServer(t *testing.T, delay time.Duration) *reproServer {
	t.Helper()

	return startReproServer(t, func(conn net.Conn, reader *bufio.Reader) {
		time.Sleep(delay)
	})
}

func startMalformedResponseServer(t *testing.T) *reproServer {
	t.Helper()

	return startReproServer(t, func(conn net.Conn, reader *bufio.Reader) {
		if _, err := io.WriteString(conn, "NOPE\r\n\r\n"); err != nil {
			t.Errorf("write malformed response: %v", err)
		}
	})
}

func startDelayedResponseServer(t *testing.T, delay time.Duration, response string) *reproServer {
	t.Helper()

	return startReproServer(t, func(conn net.Conn, reader *bufio.Reader) {
		time.Sleep(delay)
		if _, err := io.WriteString(conn, response); err != nil {
			t.Errorf("write delayed response: %v", err)
		}
	})
}

func startReproServer(t *testing.T, handler func(net.Conn, *bufio.Reader)) *reproServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		listener.Close()
	})

	server := &reproServer{
		address:  listener.Addr().String(),
		requests: make(chan string, 1),
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Errorf("set deadline: %v", err)
			return
		}

		reader := bufio.NewReader(conn)
		request, err := readHeaderBlock(reader)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		server.requests <- request

		handler(conn, reader)
	}()

	return server
}

func (s *reproServer) Address() string {
	return s.address
}

func (s *reproServer) Request(t *testing.T) string {
	t.Helper()

	select {
	case request := <-s.requests:
		return request
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reproduced request")
	}
	return ""
}
