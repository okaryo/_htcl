package http1

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"
)

type Client struct {
	Timeout time.Duration
}

type Connection struct {
	conn     net.Conn
	timeout  time.Duration
	reusable bool
}

func NewConnection(conn net.Conn, timeout time.Duration) *Connection {
	return &Connection{
		conn:     conn,
		timeout:  timeout,
		reusable: true,
	}
}

func (c *Connection) Reusable() bool {
	return c != nil && c.conn != nil && c.reusable
}

func (c Client) Do(address string, request *Request) (*Response, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	request = request.Clone()
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	request.SetHeader("Connection", "close")

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial tcp %s: %w", address, err)
	}
	defer conn.Close()

	return NewConnection(conn, timeout).RoundTrip(request)
}

func (c *Connection) RoundTrip(request *Request) (*Response, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}
	c.reusable = false

	timeout := c.timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var requestBytes bytes.Buffer
	if err := WriteRequest(&requestBytes, request); err != nil {
		return nil, fmt.Errorf("serialize HTTP request: %w", err)
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set write deadline: %w", err)
	}
	if err := writeAll(c.conn, requestBytes.Bytes()); err != nil {
		return nil, fmt.Errorf("write HTTP request: %w", err)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	response, err := ReadResponse(c.conn)
	if err != nil {
		return nil, fmt.Errorf("read HTTP response: %w", err)
	}

	c.reusable = !HasConnectionToken(request.HeaderFields, "close") && !response.ShouldCloseConnection()
	return response, nil
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}
