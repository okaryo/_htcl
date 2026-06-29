package http1

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"
)

type Client struct {
	Timeout     time.Duration
	IdleTimeout time.Duration
	TLSConfig   *tls.Config
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

type TLSInfo struct {
	Version                string
	CipherSuite            string
	ServerName             string
	NegotiatedProtocol     string
	PeerCertificateCount   int
	PeerCertificateSubject string
	VerifiedChains         int
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
	return c.DoContext(context.Background(), address, request)
}

func (c Client) DoContext(ctx context.Context, address string, request *Request) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

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
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial tcp %s: %w", address, err)
	}
	defer conn.Close()

	return NewConnection(conn, timeout).RoundTripContext(ctx, request)
}

func (c Client) DoTLS(address, serverName string, request *Request) (*Response, error) {
	response, _, err := c.DoTLSContextWithInfo(context.Background(), address, serverName, request)
	return response, err
}

func (c Client) DoTLSContext(ctx context.Context, address, serverName string, request *Request) (*Response, error) {
	response, _, err := c.DoTLSContextWithInfo(ctx, address, serverName, request)
	return response, err
}

func (c Client) DoTLSWithInfo(address, serverName string, request *Request) (*Response, TLSInfo, error) {
	return c.DoTLSContextWithInfo(context.Background(), address, serverName, request)
}

func (c Client) DoTLSContextWithInfo(ctx context.Context, address, serverName string, request *Request) (*Response, TLSInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	request = request.Clone()
	if request == nil {
		return nil, TLSInfo{}, fmt.Errorf("request is nil")
	}
	request.SetHeader("Connection", "close")

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, TLSInfo{}, fmt.Errorf("dial tcp %s: %w", address, err)
	}
	defer conn.Close()

	config, err := c.tlsConfig(serverName)
	if err != nil {
		return nil, TLSInfo{}, err
	}
	tlsConn := tls.Client(conn, config)
	if err := tlsConn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, TLSInfo{}, fmt.Errorf("set tls deadline: %w", err)
	}
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, TLSInfo{}, fmt.Errorf("tls handshake canceled: %w", ctxErr)
		}
		return nil, TLSInfo{}, fmt.Errorf("tls handshake with %s: %w", serverName, err)
	}

	info := TLSInfoFromConnectionState(tlsConn.ConnectionState())
	if info.ServerName == "" {
		info.ServerName = serverName
	}
	response, err := NewConnection(tlsConn, timeout).RoundTripContext(ctx, request)
	if err != nil {
		return nil, info, err
	}
	return response, info, nil
}

func (c *Client) DoReusable(address string, request *Request) (*Response, error) {
	return c.DoReusableContext(context.Background(), address, request)
}

func (c *Client) DoReusableContext(ctx context.Context, address string, request *Request) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.idle == nil {
		c.idle = make(map[string]idleConnection)
	}

	connection := c.takeIdle(address)
	if connection == nil {
		var err error
		connection, err = c.dialContext(ctx, address)
		if err != nil {
			return nil, err
		}
	}

	response, err := connection.RoundTripContext(ctx, request)
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

func (c *Client) idleConnectionCount() int {
	return len(c.idle)
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

func (c Client) tlsConfig(serverName string) (*tls.Config, error) {
	if serverName == "" {
		return nil, fmt.Errorf("TLS server name is required")
	}

	config := &tls.Config{
		ServerName: serverName,
		NextProtos: []string{
			"http/1.1",
		},
	}
	if c.TLSConfig != nil {
		config = c.TLSConfig.Clone()
		if config.ServerName == "" {
			config.ServerName = serverName
		}
		if len(config.NextProtos) == 0 {
			config.NextProtos = []string{"http/1.1"}
		}
	}
	return config, nil
}

func TLSInfoFromConnectionState(state tls.ConnectionState) TLSInfo {
	info := TLSInfo{
		Version:              tls.VersionName(state.Version),
		CipherSuite:          tls.CipherSuiteName(state.CipherSuite),
		ServerName:           state.ServerName,
		NegotiatedProtocol:   state.NegotiatedProtocol,
		PeerCertificateCount: len(state.PeerCertificates),
		VerifiedChains:       len(state.VerifiedChains),
	}
	if len(state.PeerCertificates) > 0 {
		info.PeerCertificateSubject = state.PeerCertificates[0].Subject.String()
	}
	return info
}

func (c *Client) dialContext(ctx context.Context, address string) (*Connection, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial tcp %s: %w", address, err)
	}
	return NewConnection(conn, timeout), nil
}

func (c *Connection) RoundTrip(request *Request) (*Response, error) {
	return c.RoundTripContext(context.Background(), request)
}

func (c *Connection) RoundTripContext(ctx context.Context, request *Request) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}
	c.reusable = false
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	timeout := c.timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	if err := request.prepare(); err != nil {
		return nil, fmt.Errorf("serialize HTTP request: %w", err)
	}

	stopCancelWatch := closeOnCancel(ctx, c.cancelClosers(request)...)
	defer stopCancelWatch()

	if err := c.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set write deadline: %w", err)
	}
	if err := WriteRequest(c.conn, request); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("request canceled: %w", ctxErr)
		}
		return nil, fmt.Errorf("write HTTP request: %w", err)
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}
	response, err := ReadResponse(c.conn)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("request canceled: %w", ctxErr)
		}
		return nil, fmt.Errorf("read HTTP response: %w", err)
	}

	c.reusable = !HasConnectionToken(request.HeaderFields, "close") && !response.ShouldCloseConnection()
	return response, nil
}

func (c *Connection) cancelClosers(request *Request) []io.Closer {
	closers := []io.Closer{c.conn}
	if request != nil {
		if closer, ok := request.BodyReader.(io.Closer); ok {
			closers = append(closers, closer)
		}
	}
	return closers
}

func closeOnCancel(ctx context.Context, closers ...io.Closer) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			for _, closer := range closers {
				if closer != nil {
					closer.Close()
				}
			}
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
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
