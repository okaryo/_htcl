package main

import (
	"flag"
	"fmt"
	"io"
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
	rawURL := flags.String("url", "", "HTTP URL to request")
	timeout := flags.Duration("timeout", 30*time.Second, "deadline used for dial, write, and read")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *rawURL == "" && flags.NArg() > 0 {
		*rawURL = flags.Arg(0)
	}
	if *rawURL != "" {
		return getURL(*rawURL, *timeout, stdout, stderr)
	}

	if *host == "" {
		*host = *address
	}

	request, err := http1.NewRequest("GET", *target, []http1.HeaderField{
		{Name: "Host", Value: *host},
		{Name: "User-Agent", Value: "htcl/0.1"},
	}, nil)
	if err != nil {
		return err
	}

	return getHTTP(*address, request, *timeout, stdout, stderr)
}

func getURL(rawURL string, timeout time.Duration, stdout, stderr io.Writer) error {
	u, err := http1.ParseURL(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		return fmt.Errorf("https URLs require TLS support, which is not implemented yet")
	}

	address, err := http1.TCPAddressForURL(u)
	if err != nil {
		return err
	}
	request, err := http1.NewRequestForURL("GET", u, []http1.HeaderField{
		{Name: "User-Agent", Value: "htcl/0.1"},
	}, nil)
	if err != nil {
		return err
	}

	return getHTTP(address, request, timeout, stdout, stderr)
}

func getHTTP(address string, request *http1.Request, timeout time.Duration, stdout, stderr io.Writer) error {
	fmt.Fprintf(stderr, "dialing tcp %s\n", address)
	fmt.Fprintln(stderr, "writing HTTP request")
	fmt.Fprintln(stderr, "reading HTTP response")

	client := http1.Client{Timeout: timeout}
	response, err := client.Do(address, request)
	if err != nil {
		return err
	}

	return writeResponse(stdout, response)
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
