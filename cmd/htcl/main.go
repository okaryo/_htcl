package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
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
	method := flags.String("method", "GET", "HTTP method")
	body := flags.String("body", "", "HTTP request body as a literal string")
	output := flags.String("output", "response", "response output mode: response, body, headers, or status")
	timeout := flags.Duration("timeout", 30*time.Second, "deadline used for dial, write, and read")
	var headers headerFlags
	flags.Var(&headers, "header", "HTTP request header in 'Name: value' form; can be repeated")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := validateOutputMode(*output); err != nil {
		return err
	}

	if *rawURL == "" && flags.NArg() > 0 {
		*rawURL = flags.Arg(0)
	}
	if *rawURL != "" {
		return getURL(*rawURL, *method, headers, []byte(*body), *output, *timeout, stdout, stderr)
	}

	if *host == "" {
		*host = *address
	}

	request, err := http1.NewRequest(*method, *target, requestHeaderFields(*host, headers), []byte(*body))
	if err != nil {
		return err
	}

	return getHTTP(*address, request, *output, *timeout, stdout, stderr)
}

func getURL(rawURL, method string, headers []http1.HeaderField, body []byte, output string, timeout time.Duration, stdout, stderr io.Writer) error {
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
	host, err := http1.HostHeaderForURL(u)
	if err != nil {
		return err
	}
	target, err := http1.RequestTargetForURL(u)
	if err != nil {
		return err
	}
	request, err := http1.NewRequest(method, target, requestHeaderFields(host, headers), body)
	if err != nil {
		return err
	}

	return getHTTP(address, request, output, timeout, stdout, stderr)
}

func getHTTP(address string, request *http1.Request, output string, timeout time.Duration, stdout, stderr io.Writer) error {
	fmt.Fprintf(stderr, "dialing tcp %s\n", address)
	fmt.Fprintln(stderr, "writing HTTP request")
	fmt.Fprintln(stderr, "reading HTTP response")

	client := http1.Client{Timeout: timeout}
	response, err := client.Do(address, request)
	if err != nil {
		return err
	}

	return writeResponse(stdout, response, output)
}

type headerFlags []http1.HeaderField

func (h *headerFlags) String() string {
	if h == nil {
		return ""
	}
	var values []string
	for _, field := range *h {
		values = append(values, field.Name+": "+field.Value)
	}
	return strings.Join(values, ", ")
}

func (h *headerFlags) Set(value string) error {
	name, fieldValue, ok := strings.Cut(value, ":")
	if !ok {
		return fmt.Errorf("header must use Name: value form")
	}
	*h = append(*h, http1.HeaderField{
		Name:  strings.TrimSpace(name),
		Value: strings.TrimSpace(fieldValue),
	})
	return nil
}

func requestHeaderFields(host string, custom []http1.HeaderField) []http1.HeaderField {
	fields := []http1.HeaderField{
		{Name: "Host", Value: host},
		{Name: "User-Agent", Value: "htcl/0.1"},
	}
	for _, field := range custom {
		setHeaderField(&fields, field)
	}
	return fields
}

func setHeaderField(fields *[]http1.HeaderField, next http1.HeaderField) {
	for i, field := range *fields {
		if strings.EqualFold(field.Name, next.Name) {
			(*fields)[i] = next
			return
		}
	}
	*fields = append(*fields, next)
}

func validateOutputMode(output string) error {
	switch output {
	case "response", "body", "headers", "status":
		return nil
	default:
		return fmt.Errorf("unsupported output mode %q", output)
	}
}

func writeResponse(w io.Writer, response *http1.Response, output string) error {
	switch output {
	case "response":
		if err := writeStatusLine(w, response); err != nil {
			return err
		}
		if err := writeHeaderFields(w, response.HeaderFields); err != nil {
			return err
		}
		if _, err := fmt.Fprint(w, "\r\n"); err != nil {
			return err
		}
		if _, err := w.Write(response.Body); err != nil {
			return err
		}
		return nil
	case "body":
		_, err := w.Write(response.Body)
		return err
	case "headers":
		if err := writeStatusLine(w, response); err != nil {
			return err
		}
		if err := writeHeaderFields(w, response.HeaderFields); err != nil {
			return err
		}
		_, err := fmt.Fprint(w, "\r\n")
		return err
	case "status":
		return writeStatusLine(w, response)
	default:
		return fmt.Errorf("unsupported output mode %q", output)
	}
}

func writeStatusLine(w io.Writer, response *http1.Response) error {
	if _, err := fmt.Fprintf(w, "%s %03d %s\r\n", response.Version, response.StatusCode, response.ReasonPhrase); err != nil {
		return err
	}
	return nil
}

func writeHeaderFields(w io.Writer, fields []http1.HeaderField) error {
	for _, field := range fields {
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", field.Name, field.Value); err != nil {
			return err
		}
	}
	return nil
}
