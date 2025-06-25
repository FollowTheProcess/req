// Package req implements the actual functionality exposed via the CLI.
package req

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"go.followtheprocess.codes/hue"
	"go.followtheprocess.codes/log"
	"go.followtheprocess.codes/msg"
	"go.followtheprocess.codes/req/internal/spec"
	"go.followtheprocess.codes/req/internal/syntax"
	"go.followtheprocess.codes/req/internal/syntax/parser"
)

// Styles.
const (
	headerName = hue.Cyan
	success    = hue.Green | hue.Bold
	failure    = hue.Red | hue.Bold
)

// HTTP config.
const (
	// DefaultTimeout is the default amount of time allowed for the entire request cycle.
	DefaultTimeout = 30 * time.Second

	// DefaultConnectionTimeout is the default amount of time allowed for the HTTP connection/TLS handshake.
	DefaultConnectionTimeout = 10 * time.Second

	keepAliveTimeout      = 30 * time.Second
	idleTimeout           = 90 * time.Second
	expectContinueTimeout = 1 * time.Second
	maxIdleConns          = 100
)

// TODO(@FollowTheProcess): A command that takes an OpenAPI schema and dumps it to .http file(s)
// TODO(@FollowTheProcess): Can we syntax highlight the JSON body? I guess look at Content-Type and decide from that

// Req holds the state of the program.
type Req struct {
	stdout io.Writer   // Normal program output is written here
	stderr io.Writer   // Errors, logs and debug info written here
	logger *log.Logger // The logger, passed around the whole program
}

// New returns a new instance of [Req].
func New(stdout, stderr io.Writer, debug bool) Req {
	level := log.LevelInfo
	if debug {
		level = log.LevelDebug
	}

	logger := log.New(stderr, log.WithLevel(level))
	return Req{
		stdout: stdout,
		stderr: stderr,
		logger: logger,
	}
}

// CheckOptions are the flags passed to the check subcommand.
type CheckOptions struct {
	Verbose bool // Enable debug logs
}

// Check implements the `req check` subcommand.
func (r Req) Check(files []string, options CheckOptions) error {
	logger := r.logger.Prefixed("check")
	overallStart := time.Now()

	for _, file := range files {
		logger.Debug("Checking", "file", file)
		start := time.Now()
		f, err := os.Open(file)
		if err != nil {
			return err
		}

		parser, err := parser.New(file, f, syntax.PrettyConsoleHandler(r.stderr))
		if err != nil {
			return err
		}

		_, err = parser.Parse()
		if err != nil {
			return fmt.Errorf("%w: %s is not valid http syntax", err, file)
		}

		f.Close()

		msg.Fsuccess(r.stdout, "%s is valid", file)
		logger.Debug("Took", "duration", time.Since(start))
	}

	logger.Debug("Took (overall)", "duration", time.Since(overallStart))
	return nil
}

// ShowOptions are the flags passed to the `req show` subcommand.
type ShowOptions struct {
	Resolve bool // Resolve variables and do replacements
	JSON    bool // Output the file in JSON
	Verbose bool // Enable debug logs
}

// Show implements the `req show` subcommand.
func (r Req) Show(file string, options ShowOptions) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	parser, err := parser.New(file, f, syntax.PrettyConsoleHandler(r.stderr))
	if err != nil {
		return err
	}

	raw, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("%w: %s is not valid http syntax", err, file)
	}

	if options.Resolve {
		resolved, err := spec.ResolveFile(raw)
		if err != nil {
			return err
		}

		if options.JSON {
			return json.NewEncoder(r.stdout).Encode(resolved)
		}

		fmt.Fprintln(r.stdout, strings.TrimSpace(resolved.String()))
		return nil
	}

	if options.JSON {
		return json.NewEncoder(r.stdout).Encode(raw)
	}

	fmt.Fprintln(r.stdout, strings.TrimSpace(raw.String()))
	return nil
}

// DoOptions are the flags passed to the `req do` subcommand.
type DoOptions struct {
	Output            string
	Timeout           time.Duration
	ConnectionTimeout time.Duration
	NoRedirect        bool
	Verbose           bool
}

// Do implements the `req do` subcommand.
func (r Req) Do(file, name string, options DoOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	logger := r.logger.Prefixed("do").With("file", file, "request", name)
	parseStart := time.Now()

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	parser, err := parser.New(file, f, syntax.PrettyConsoleHandler(r.stderr))
	if err != nil {
		return err
	}

	raw, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("%w: %s is not valid http syntax", err, file)
	}

	resolved, err := spec.ResolveFile(raw)
	if err != nil {
		return err
	}

	request, ok := resolved.GetRequest(name)
	if !ok {
		return fmt.Errorf("%s does not contain request %s", file, name)
	}

	logger.Debug("Parsed file", "duration", time.Since(parseStart))

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		request.Method,
		request.URL,
		bytes.NewReader(request.Body),
	)
	if err != nil {
		return err
	}

	for key, value := range request.Headers {
		httpRequest.Header.Add(key, value)
	}

	client := httpClient(request)

	requestStart := time.Now()
	logger.Debug(
		"Sending HTTP request",
		"method",
		request.Method,
		"url",
		request.URL,
		"headers",
		request.Headers,
	)

	response, err := client.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("HTTP: %w", err)
	}

	if response == nil {
		return errors.New("nil response")
	}

	defer response.Body.Close()

	logger.Debug("Response", "status", response.Status, "duration", time.Since(requestStart))

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode >= http.StatusBadRequest {
		fmt.Fprintln(r.stdout, failure.Text(response.Status))
	} else {
		fmt.Fprintln(r.stdout, success.Text(response.Status))
	}

	for _, key := range slices.Sorted(maps.Keys(response.Header)) {
		fmt.Fprintf(r.stdout, "%s: %s\n", headerName.Text(key), response.Header.Get(key))
	}

	fmt.Fprintln(r.stdout) // Line space

	fmt.Fprintln(r.stdout, string(body))
	return nil
}

// construct a HTTP client customised for the request with timeouts, no redirect policies etc.
func httpClient(request spec.Request) *http.Client {
	var checkRedirect func(req *http.Request, via []*http.Request) error
	if request.NoRedirect {
		checkRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	dialContext := func(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext
	}

	// We want to always try HTTP2 unless opted out, this is the behaviour
	// of the default std lib http client anyway
	http2 := !strings.HasPrefix(request.HTTPVersion, "HTTP/1")

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: dialContext(&net.Dialer{
				Timeout:   request.Timeout,
				KeepAlive: keepAliveTimeout,
			}),
			ForceAttemptHTTP2:     http2,
			MaxIdleConns:          maxIdleConns,
			IdleConnTimeout:       idleTimeout,
			TLSHandshakeTimeout:   request.ConnectionTimeout,
			ExpectContinueTimeout: expectContinueTimeout,
		},
		CheckRedirect: checkRedirect,
		Timeout:       request.Timeout,
	}

	return client
}
