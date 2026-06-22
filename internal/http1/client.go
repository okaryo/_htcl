package http1

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"
)

type Client struct {
	Timeout     time.Duration
	IdleTimeout time.Duration
	idle        map[string]idleConnection
	now         func() time.Time
}

type idleConnection struct {
	connection *Connection
	idleAt     time.Time
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

func (c *Client) DoReusable(address string, request *Request) (*Response, error) {
	if c.idle == nil {
		c.idle = make(map[string]idleConnection)
	}

	connection := c.takeIdle(address)
	if connection == nil {
		var err error
		connection, err = c.dial(address)
		if err != nil {
			return nil, err
		}
	}

	response, err := connection.RoundTrip(request)
	if err != nil {
		connection.Close()
		return nil, err
	}
	if connection.Reusable() {
		c.idle[address] = idleConnection{
			connection: connection,
			idleAt:     c.currentTime(),
		}
	} else {
		connection.Close()
	}

	return response, nil
}

func (c *Client) CloseIdleConnections() {
	for address, idle := range c.idle {
		idle.connection.Close()
		delete(c.idle, address)
	}
}

func (c *Client) takeIdle(address string) *Connection {
	idle, ok := c.idle[address]
	if !ok {
		return nil
	}
	delete(c.idle, address)

	if idle.connection == nil || !idle.connection.Reusable() {
		if idle.connection != nil {
			idle.connection.Close()
		}
		return nil
	}
	if c.idleExpired(idle.idleAt) {
		idle.connection.Close()
		return nil
	}

	return idle.connection
}

func (c *Client) idleExpired(idleAt time.Time) bool {
	timeout := c.IdleTimeout
	if timeout == 0 {
		timeout = 90 * time.Second
	}
	return !idleAt.IsZero() && !c.currentTime().Before(idleAt.Add(timeout))
}

func (c *Client) currentTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *Client) dial(address string) (*Connection, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial tcp %s: %w", address, err)
	}
	return NewConnection(conn, timeout), nil
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

func (c *Connection) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.reusable = false
	err := c.conn.Close()
	c.conn = nil
	return err
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
