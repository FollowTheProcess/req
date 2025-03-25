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
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/FollowTheProcess/hue"
	"github.com/FollowTheProcess/msg"
	"github.com/FollowTheProcess/req/internal/spec"
	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/parser"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// Styles.
const (
	headerName = hue.Cyan
	success    = hue.Green
	failure    = hue.Red
)

// TODO(@FollowTheProcess): A command that takes an OpenAPI schema and dumps it to .http file(s)

// Req holds the state of the program.
type Req struct {
	stdout io.Writer
	stderr io.Writer
	logger *log.Logger
}

// New returns a new instance of [Req].
func New(stdout, stderr io.Writer, debug bool) Req {
	const width = 5

	level := log.InfoLevel
	if debug {
		level = log.DebugLevel
	}

	logger := log.NewWithOptions(stderr, log.Options{
		ReportTimestamp: true,
		Level:           level,
	})

	// Largely the default styles but with a default MaxWidth of 5 so as to not cutoff
	// DEBUG or ERROR
	logger.SetStyles(&log.Styles{
		Timestamp: lipgloss.NewStyle(),
		Caller:    lipgloss.NewStyle().Faint(true),
		Prefix:    lipgloss.NewStyle().Bold(true).Faint(true),
		Message:   lipgloss.NewStyle(),
		Key:       lipgloss.NewStyle().Faint(true),
		Value:     lipgloss.NewStyle(),
		Separator: lipgloss.NewStyle().Faint(true),
		Levels: map[log.Level]lipgloss.Style{
			log.DebugLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(log.DebugLevel.String())).
				Bold(true).
				MaxWidth(width).
				Foreground(lipgloss.Color("63")),
			log.InfoLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(log.InfoLevel.String())).
				Bold(true).
				MaxWidth(width).
				Foreground(lipgloss.Color("86")),
			log.WarnLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(log.WarnLevel.String())).
				Bold(true).
				MaxWidth(width).
				Foreground(lipgloss.Color("192")),
			log.ErrorLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(log.ErrorLevel.String())).
				Bold(true).
				MaxWidth(width).
				Foreground(lipgloss.Color("204")),
			log.FatalLevel: lipgloss.NewStyle().
				SetString(strings.ToUpper(log.FatalLevel.String())).
				Bold(true).
				MaxWidth(width).
				Foreground(lipgloss.Color("134")),
		},
		Keys:   map[string]lipgloss.Style{},
		Values: map[string]lipgloss.Style{},
	})

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
	logger := r.logger.WithPrefix("check")
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
	logger := r.logger.WithPrefix("do").With("file", file, "request", name)
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

	// TODO(@FollowTheProcess): A context with a timeout and listens to ctrl+c
	httpRequest, err := http.NewRequestWithContext(
		context.TODO(),
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

	// TODO(@FollowTheProcess): Make a proper http client elsewhere
	client := http.Client{
		Timeout: request.Timeout,
	}

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

	fmt.Fprintln(r.stdout, string(body))
	return nil
}
