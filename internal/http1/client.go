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

	var requestBytes bytes.Buffer
	if err := WriteRequest(&requestBytes, request); err != nil {
		return nil, fmt.Errorf("serialize HTTP request: %w", err)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set write deadline: %w", err)
	}
	if err := writeAll(conn, requestBytes.Bytes()); err != nil {
		return nil, fmt.Errorf("write HTTP request: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	response, err := ReadResponse(conn)
	if err != nil {
		return nil, fmt.Errorf("read HTTP response: %w", err)
	}

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
