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
	DebugLog    DebugLogger
	idle        map[string]idleConnection
	now         func() time.Time
}

type idleConnection struct {
	connection *Connection
	idleAt     time.Time
}

type Connection struct {
	conn         net.Conn
	timeout      time.Duration
	reusable     bool
	debugLog     DebugLogger
	debugAddress string
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

	request = request.Clone()
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	request.SetHeader("Connection", "close")

	connection, err := (&c).dialContext(ctx, address)
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	return connection.RoundTripContext(ctx, request)
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
	c.emitDebug(DebugEvent{
		Name:    DebugEventDialStart,
		Phase:   ErrorPhaseDial,
		Address: address,
	})
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		err = classifyClientError(ErrorPhaseDial, fmt.Errorf("dial tcp %s: %w", address, err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventDialDone,
			Phase:   ErrorPhaseDial,
			Address: address,
			Err:     err,
		})
		return nil, TLSInfo{}, err
	}
	defer conn.Close()
	c.emitDebug(DebugEvent{
		Name:    DebugEventDialDone,
		Phase:   ErrorPhaseDial,
		Address: address,
	})

	config, err := c.tlsConfig(serverName)
	if err != nil {
		return nil, TLSInfo{}, err
	}
	tlsConn := tls.Client(conn, config)
	c.emitDebug(DebugEvent{
		Name:    DebugEventTLSHandshakeStart,
		Phase:   ErrorPhaseTLSHandshake,
		Address: address,
	})
	if err := tlsConn.SetDeadline(time.Now().Add(timeout)); err != nil {
		err = classifyClientError(ErrorPhaseTLSHandshake, fmt.Errorf("set tls deadline: %w", err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventTLSHandshakeDone,
			Phase:   ErrorPhaseTLSHandshake,
			Address: address,
			Err:     err,
		})
		return nil, TLSInfo{}, err
	}
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = classifyClientError(ErrorPhaseTLSHandshake, fmt.Errorf("tls handshake canceled: %w", ctxErr))
			c.emitDebug(DebugEvent{
				Name:    DebugEventTLSHandshakeDone,
				Phase:   ErrorPhaseTLSHandshake,
				Address: address,
				Err:     err,
			})
			return nil, TLSInfo{}, err
		}
		err = classifyClientError(ErrorPhaseTLSHandshake, fmt.Errorf("tls handshake with %s: %w", serverName, err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventTLSHandshakeDone,
			Phase:   ErrorPhaseTLSHandshake,
			Address: address,
			Err:     err,
		})
		return nil, TLSInfo{}, err
	}
	c.emitDebug(DebugEvent{
		Name:    DebugEventTLSHandshakeDone,
		Phase:   ErrorPhaseTLSHandshake,
		Address: address,
	})

	info := TLSInfoFromConnectionState(tlsConn.ConnectionState())
	if info.ServerName == "" {
		info.ServerName = serverName
	}
	connection := NewConnection(tlsConn, timeout)
	c.configureConnection(connection, address)
	response, err := connection.RoundTripContext(ctx, request)
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
	} else {
		c.configureConnection(connection, address)
		c.emitDebug(DebugEvent{
			Name:    DebugEventConnectionReused,
			Address: address,
		})
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
		c.emitDebug(DebugEvent{
			Name:     DebugEventConnectionIdle,
			Address:  address,
			Reusable: true,
		})
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

func (c *Client) configureConnection(connection *Connection, address string) {
	if connection == nil {
		return
	}
	connection.debugLog = c.DebugLog
	connection.debugAddress = address
}

func (c *Client) emitDebug(event DebugEvent) {
	if c == nil || c.DebugLog == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = c.currentTime()
	}
	c.DebugLog(event)
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
	c.emitDebug(DebugEvent{
		Name:    DebugEventDialStart,
		Phase:   ErrorPhaseDial,
		Address: address,
	})
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		err = classifyClientError(ErrorPhaseDial, fmt.Errorf("dial tcp %s: %w", address, err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventDialDone,
			Phase:   ErrorPhaseDial,
			Address: address,
			Err:     err,
		})
		return nil, err
	}
	c.emitDebug(DebugEvent{
		Name:    DebugEventDialDone,
		Phase:   ErrorPhaseDial,
		Address: address,
	})
	connection := NewConnection(conn, timeout)
	c.configureConnection(connection, address)
	return connection, nil
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
		return nil, classifyClientError(ErrorPhaseSerialize, fmt.Errorf("serialize HTTP request: %w", err))
	}

	stopCancelWatch := closeOnCancel(ctx, c.cancelClosers(request)...)
	defer stopCancelWatch()

	c.emitDebug(DebugEvent{
		Name:    DebugEventWriteRequestStart,
		Phase:   ErrorPhaseWriteRequest,
		Address: c.debugAddress,
		Method:  request.Method,
		Target:  request.Target,
	})
	if err := c.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		err = classifyClientError(ErrorPhaseWriteRequest, fmt.Errorf("set write deadline: %w", err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventWriteRequestDone,
			Phase:   ErrorPhaseWriteRequest,
			Address: c.debugAddress,
			Method:  request.Method,
			Target:  request.Target,
			Err:     err,
		})
		return nil, err
	}
	if err := WriteRequest(c.conn, request); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = classifyClientError(ErrorPhaseWriteRequest, fmt.Errorf("request canceled: %w", ctxErr))
			c.emitDebug(DebugEvent{
				Name:    DebugEventWriteRequestDone,
				Phase:   ErrorPhaseWriteRequest,
				Address: c.debugAddress,
				Method:  request.Method,
				Target:  request.Target,
				Err:     err,
			})
			return nil, err
		}
		err = classifyClientError(ErrorPhaseWriteRequest, fmt.Errorf("write HTTP request: %w", err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventWriteRequestDone,
			Phase:   ErrorPhaseWriteRequest,
			Address: c.debugAddress,
			Method:  request.Method,
			Target:  request.Target,
			Err:     err,
		})
		return nil, err
	}
	c.emitDebug(DebugEvent{
		Name:    DebugEventWriteRequestDone,
		Phase:   ErrorPhaseWriteRequest,
		Address: c.debugAddress,
		Method:  request.Method,
		Target:  request.Target,
	})

	c.emitDebug(DebugEvent{
		Name:    DebugEventReadResponseStart,
		Phase:   ErrorPhaseReadResponse,
		Address: c.debugAddress,
		Method:  request.Method,
		Target:  request.Target,
	})
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		err = classifyClientError(ErrorPhaseReadResponse, fmt.Errorf("set read deadline: %w", err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventReadResponseDone,
			Phase:   ErrorPhaseReadResponse,
			Address: c.debugAddress,
			Method:  request.Method,
			Target:  request.Target,
			Err:     err,
		})
		return nil, err
	}
	response, err := ReadResponse(c.conn)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = classifyClientError(ErrorPhaseReadResponse, fmt.Errorf("request canceled: %w", ctxErr))
			c.emitDebug(DebugEvent{
				Name:    DebugEventReadResponseDone,
				Phase:   ErrorPhaseReadResponse,
				Address: c.debugAddress,
				Method:  request.Method,
				Target:  request.Target,
				Err:     err,
			})
			return nil, err
		}
		err = classifyClientError(ErrorPhaseReadResponse, fmt.Errorf("read HTTP response: %w", err))
		c.emitDebug(DebugEvent{
			Name:    DebugEventReadResponseDone,
			Phase:   ErrorPhaseReadResponse,
			Address: c.debugAddress,
			Method:  request.Method,
			Target:  request.Target,
			Err:     err,
		})
		return nil, err
	}

	c.reusable = !HasConnectionToken(request.HeaderFields, "close") && !response.ShouldCloseConnection()
	c.emitDebug(DebugEvent{
		Name:       DebugEventReadResponseDone,
		Phase:      ErrorPhaseReadResponse,
		Address:    c.debugAddress,
		Method:     request.Method,
		Target:     request.Target,
		StatusCode: response.StatusCode,
		Reusable:   c.reusable,
	})
	return response, nil
}

func (c *Connection) emitDebug(event DebugEvent) {
	if c == nil || c.debugLog == nil {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	c.debugLog(event)
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
