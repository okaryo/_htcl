package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/okaryo/_htcl/internal/http1"
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

	return getHTTP(*address, *host, *target, *timeout, stdout, stderr)
}

func getHTTP(address, host, target string, timeout time.Duration, stdout, stderr io.Writer) error {
	dialer := net.Dialer{Timeout: timeout}

	fmt.Fprintf(stderr, "dialing tcp %s\n", address)
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("dial tcp %s: %w", address, err)
	}
	defer conn.Close()

	request, err := http1.NewRequest("GET", target, []http1.HeaderField{
		{Name: "Host", Value: host},
		{Name: "User-Agent", Value: "htcl/0.1"},
		{Name: "Connection", Value: "close"},
	}, nil)
	if err != nil {
		return err
	}

	var requestBytes bytes.Buffer
	if err := http1.WriteRequest(&requestBytes, request); err != nil {
		return fmt.Errorf("serialize HTTP request: %w", err)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	fmt.Fprintf(stderr, "writing HTTP request (%d bytes)\n", requestBytes.Len())
	if err := writeAll(conn, requestBytes.Bytes()); err != nil {
		return fmt.Errorf("write HTTP request: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	fmt.Fprintln(stderr, "reading HTTP response")
	response, err := http1.ReadResponse(conn)
	if err != nil {
		return fmt.Errorf("read HTTP response: %w", err)
	}

	return writeResponse(stdout, response)
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

func writeResponse(w io.Writer, response *http1.Response) error {
	if _, err := fmt.Fprintf(w, "%s %03d %s\r\n", response.Version, response.StatusCode, response.ReasonPhrase); err != nil {
		return err
	}
	for _, field := range response.HeaderFields {
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", field.Name, field.Value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w, "\r\n"); err != nil {
		return err
	}
	if _, err := w.Write(response.Body); err != nil {
		return err
	}
	return nil
}
