package parser_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.followtheprocess.codes/req/internal/syntax"
	"go.followtheprocess.codes/req/internal/syntax/parser"
	"go.followtheprocess.codes/test"
	"go.followtheprocess.codes/txtar"
	"go.uber.org/goleak"
)

var update = flag.Bool("update", false, "Update snapshots and testdata")

// TestValid is the primary parser test for valid syntax. It reads src http text from
// a txtar archive in testdata/valid, parses it to completion, serialises that parsed result
// to JSON then generates a pretty diff if it doesn't match.
func TestValid(t *testing.T) {
	test.ColorEnabled(true) // Force colour in the diffs

	pattern := filepath.Join("testdata", "valid", "*.txtar")
	files, err := filepath.Glob(pattern)
	test.Ok(t, err)

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			defer goleak.VerifyNone(t)

			archive, err := txtar.ParseFile(file)
			test.Ok(t, err)

			src, ok := archive.Read("src.http")
			test.True(t, ok, test.Context("archive %s missing src.http", name))

			want, ok := archive.Read("want.json")
			test.True(t, ok, test.Context("archive %s missing want.json", name))

			parser, err := parser.New(name, strings.NewReader(src), testFailHandler(t))
			test.Ok(t, err)

			got, err := parser.Parse()
			test.Ok(t, err, test.Context("unexpected parse error"))

			gotJSON, err := json.MarshalIndent(got, "", "  ")
			test.Ok(t, err, test.Context("could not marshal JSON"))

			gotJSON = append(gotJSON, '\n') // MarshalIndent doesn't do newlines at the end

			if *update {
				err := archive.Write("want.json", string(gotJSON))
				test.Ok(t, err)

				err = txtar.DumpFile(file, archive)
				test.Ok(t, err)

				return
			}

			test.Diff(t, string(gotJSON), want)
		})
	}
}

// TestInvalid is the primary test for invalid syntax. It does much the same as TestParseValid
// but instead of failing tests if a syntax error is encounter, it fails if there is not any syntax errors.
//
// Additionally, the errors are compared against a reference.
func TestInvalid(t *testing.T) {
	test.ColorEnabled(true) // Force colour in the diffs

	pattern := filepath.Join("testdata", "invalid", "*.txtar")
	files, err := filepath.Glob(pattern)
	test.Ok(t, err)

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			defer goleak.VerifyNone(t)

			archive, err := txtar.ParseFile(file)
			test.Ok(t, err)

			src, ok := archive.Read("src.http")
			test.True(t, ok, test.Context("archive %s missing src.http", name))

			want, ok := archive.Read("want.txt")
			test.True(t, ok, test.Context("archive %s missing want.txt", name))

			collector := &errorCollector{}

			parser, err := parser.New(name, strings.NewReader(src), collector.handler())
			test.Ok(t, err)

			_, err = parser.Parse()
			test.Err(t, err, test.Context("Parse() failed to return an error given invalid syntax"))

			got := collector.String()

			if *update {
				err := archive.Write("want.txt", got)
				test.Ok(t, err)

				err = txtar.DumpFile(file, archive)
				test.Ok(t, err)

				return
			}

			test.Diff(t, got, want)
		})
	}
}

func FuzzParser(f *testing.F) {
	// Get all the .http source from testdata for the corpus
	pattern := filepath.Join("testdata", "valid", "*.txtar")
	files, err := filepath.Glob(pattern)
	test.Ok(f, err)

	for _, file := range files {
		archive, err := txtar.ParseFile(file)
		test.Ok(f, err)

		src, ok := archive.Read("src.http")
		test.True(f, ok, test.Context("file %s does not contain 'src.http'", file))

		f.Add(src)
	}

	// Property: The parser never panics or loops indefinitely, fuzz by default
	// will catch both of these
	f.Fuzz(func(t *testing.T, src string) {
		// Note: no ErrorHandler installed, because if we let it report errors
		// it would kill the fuzz test straight away e.g. on the first invalid
		// utf-8 char
		parser, err := parser.New("fuzz", strings.NewReader(src), nil)
		test.Ok(t, err)

		file, err := parser.Parse()

		var zeroFile syntax.File

		// Property: If the parser returned an error, then file must be empty
		if err != nil {
			if !reflect.DeepEqual(file, zeroFile) {
				t.Fatalf("\nnon zero syntax.File returned when err != nil: %#v\n", file)
			}
		}
	})
}

// testFailHandler returns a [syntax.ErrorHandler] that handles scanning errors by failing
// the enclosing test.
func testFailHandler(tb testing.TB) syntax.ErrorHandler {
	tb.Helper()

	return func(pos syntax.Position, msg string) {
		tb.Fatalf("%s: %s", pos, msg)
	}
}

type errorCollector struct {
	errs []string
	mu   sync.Mutex
}

func (e *errorCollector) String() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	errsCopy := slices.Clone(e.errs)

	var s strings.Builder

	slices.Sort(errsCopy) // Deterministic

	for _, err := range errsCopy {
		s.WriteString(err)
	}

	return s.String()
}

func (e *errorCollector) handler() syntax.ErrorHandler {
	return func(pos syntax.Position, msg string) {
		// Because the scanner runs in it's own goroutine and also makes use of the
		// handler
		e.mu.Lock()
		defer e.mu.Unlock()

		e.errs = append(e.errs, fmt.Sprintf("%s: %s\n", pos, msg))
	}
}
