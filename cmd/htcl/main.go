package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "htcl: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("htcl", flag.ContinueOnError)
	flags.SetOutput(stderr)

	address := flags.String("addr", "127.0.0.1:8080", "TCP address to connect to")
	host := flags.String("host", "", "HTTP Host header value; defaults to -addr")
	target := flags.String("target", "/", "HTTP request target")
	timeout := flags.Duration("timeout", 30*time.Second, "deadline used for dial, write, and read")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *host == "" {
		*host = *address
	}
	if *target == "" || !strings.HasPrefix(*target, "/") {
		return fmt.Errorf("target must start with /")
	}

	return getRawHTTP(*address, *host, *target, *timeout, stdout, stderr)
}

func getRawHTTP(address, host, target string, timeout time.Duration, stdout, stderr io.Writer) error {
	dialer := net.Dialer{Timeout: timeout}

	fmt.Fprintf(stderr, "dialing tcp %s\n", address)
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("dial tcp %s: %w", address, err)
	}
	defer conn.Close()

	request := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: htcl/0.1\r\nConnection: close\r\n\r\n",
		target,
		host,
	)

	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	fmt.Fprintf(stderr, "writing HTTP request (%d bytes)\n", len(request))
	if err := writeAll(conn, []byte(request)); err != nil {
		return fmt.Errorf("write HTTP request: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	fmt.Fprintln(stderr, "reading raw HTTP response")
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			if _, writeErr := stdout.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write response to stdout: %w", writeErr)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read HTTP response: %w", err)
		}
	}
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
