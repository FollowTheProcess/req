package scanner_test

import (
	"flag"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.followtheprocess.codes/req/internal/syntax"
	"go.followtheprocess.codes/req/internal/syntax/scanner"
	"go.followtheprocess.codes/req/internal/syntax/token"
	"go.followtheprocess.codes/test"
	"go.followtheprocess.codes/txtar"
	"go.uber.org/goleak"
)

var update = flag.Bool("update", false, "Update snapshots and testdata")

func TestBasics(t *testing.T) {
	tests := []struct {
		name string        // Name of the test case
		src  string        // Source text to scan
		want []token.Token // Expected token stream
	}{
		{
			name: "empty",
			src:  "",
			want: []token.Token{
				{Kind: token.EOF, Start: 0, End: 0},
			},
		},
		{
			name: "hash comment",
			src:  "# I'm a hash comment",
			want: []token.Token{
				{Kind: token.Comment, Start: 2, End: 20},
				{Kind: token.EOF, Start: 20, End: 20},
			},
		},
		{
			name: "slash comment",
			src:  "// I'm a slash comment",
			want: []token.Token{
				{Kind: token.Comment, Start: 3, End: 22},
				{Kind: token.EOF, Start: 22, End: 22},
			},
		},
		{
			name: "request separator",
			src:  "###",
			want: []token.Token{
				{Kind: token.Separator, Start: 0, End: 3},
				{Kind: token.EOF, Start: 3, End: 3},
			},
		},
		{
			name: "request separator with comment",
			src:  "### My Special Request",
			want: []token.Token{
				{Kind: token.Separator, Start: 0, End: 3},
				{Kind: token.Comment, Start: 4, End: 22},
				{Kind: token.EOF, Start: 22, End: 22},
			},
		},
		{
			name: "at",
			src:  "@",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.EOF, Start: 1, End: 1},
			},
		},
		{
			name: "variable",
			src:  "@var = test",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 4},
				{Kind: token.Eq, Start: 5, End: 6},
				{Kind: token.Text, Start: 7, End: 11},
				{Kind: token.EOF, Start: 11, End: 11},
			},
		},
		{
			name: "variable no equals",
			src:  "@var test",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 4},
				{Kind: token.Text, Start: 5, End: 9},
				{Kind: token.EOF, Start: 9, End: 9},
			},
		},
		{
			name: "name",
			src:  "@name = MyRequest",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Name, Start: 1, End: 5},
				{Kind: token.Eq, Start: 6, End: 7},
				{Kind: token.Text, Start: 8, End: 17},
				{Kind: token.EOF, Start: 17, End: 17},
			},
		},
		{
			name: "name no equals",
			src:  "@name MyRequest",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Name, Start: 1, End: 5},
				{Kind: token.Text, Start: 6, End: 15},
				{Kind: token.EOF, Start: 15, End: 15},
			},
		},
		{
			name: "hash request variable",
			src:  "# @var = test",
			want: []token.Token{
				{Kind: token.At, Start: 2, End: 3},
				{Kind: token.Ident, Start: 3, End: 6},
				{Kind: token.Eq, Start: 7, End: 8},
				{Kind: token.Text, Start: 9, End: 13},
				{Kind: token.EOF, Start: 13, End: 13},
			},
		},
		{
			name: "slash request variable",
			src:  "// @var = test",
			want: []token.Token{
				{Kind: token.At, Start: 3, End: 4},
				{Kind: token.Ident, Start: 4, End: 7},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Text, Start: 10, End: 14},
				{Kind: token.EOF, Start: 14, End: 14},
			},
		},
		{
			name: "slash request variable",
			src:  "// @var = test",
			want: []token.Token{
				{Kind: token.At, Start: 3, End: 4},
				{Kind: token.Ident, Start: 4, End: 7},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Text, Start: 10, End: 14},
				{Kind: token.EOF, Start: 14, End: 14},
			},
		},
		{
			name: "at ident equal integer",
			src:  "@something=20",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.Eq, Start: 10, End: 11},
				{Kind: token.Text, Start: 11, End: 13},
				{Kind: token.EOF, Start: 13, End: 13},
			},
		},
		{
			name: "at timeout equal duration",
			src:  "@timeout = 20s", // Space because why not
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Timeout, Start: 1, End: 8},
				{Kind: token.Eq, Start: 9, End: 10},
				{Kind: token.Text, Start: 11, End: 14},
				{Kind: token.EOF, Start: 14, End: 14},
			},
		},
		{
			name: "prompt",
			src:  "@prompt username",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Prompt, Start: 1, End: 7},
				{Kind: token.Ident, Start: 8, End: 16},
				{Kind: token.EOF, Start: 16, End: 16},
			},
		},
		{
			name: "prompt with description",
			src:  "@prompt username The name of an authenticated user",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Prompt, Start: 1, End: 7},
				{Kind: token.Ident, Start: 8, End: 16},
				{Kind: token.Text, Start: 17, End: 50},
				{Kind: token.EOF, Start: 50, End: 50},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer goleak.VerifyNone(t)

			src := []byte(tt.src)
			scanner := scanner.New(tt.name, src, testFailHandler(t))

			tokens := slices.Collect(scanner.All())

			test.EqualFunc(t, tokens, tt.want, slices.Equal, test.Context("token stream mismatch"))
		})
	}
}

func TestValid(t *testing.T) {
	test.ColorEnabled(true) // Force colour for the diffs

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
			test.True(t, ok, test.Context("archive missing src.http"))

			want, ok := archive.Read("tokens.txt")
			test.True(t, ok, test.Context("archive missing tokens.txt"))

			scanner := scanner.New(name, []byte(src), testFailHandler(t))

			tokens := slices.Collect(scanner.All())

			var formattedTokens strings.Builder
			for _, tok := range tokens {
				formattedTokens.WriteString(tok.String())
				formattedTokens.WriteByte('\n')
			}

			got := formattedTokens.String()

			if *update {
				// Update the expected with what's actually been seen
				err := archive.Write("tokens.txt", got)
				test.Ok(t, err)

				err = txtar.DumpFile(file, archive)
				test.Ok(t, err)

				return
			}

			test.Diff(t, got, want)
		})
	}
}

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
			test.True(t, ok, test.Context("archive missing src.http"))

			want, ok := archive.Read("tokens.txt")
			test.True(t, ok, test.Context("archive missing tokens.txt"))

			errs, ok := archive.Read("errors.txt")
			test.True(t, ok, test.Context("archive missing errors.txt"))

			collector := &errorCollector{}

			scanner := scanner.New(name, []byte(src), collector.handler())

			tokens := slices.Collect(scanner.All())

			var formattedTokens strings.Builder
			for _, tok := range tokens {
				formattedTokens.WriteString(tok.String())
				formattedTokens.WriteByte('\n')
			}

			got := formattedTokens.String()
			gotErrs := collector.String()

			if *update {
				// Update the expected with what's actually been seen
				err := archive.Write("tokens.txt", got)
				test.Ok(t, err)

				err = archive.Write("errors.txt", gotErrs)
				test.Ok(t, err)

				err = txtar.DumpFile(file, archive)
				test.Ok(t, err)

				return
			}

			test.Diff(t, got, want)
			test.Diff(t, gotErrs, errs)
		})
	}
}

func FuzzScanner(f *testing.F) {
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

	// Property: The scanner never panics or loops indefinitely, fuzz
	// by default will catch both of these
	f.Fuzz(func(t *testing.T, src string) {
		// Note: no ErrorHandler installed, because if we let the scanner report syntax
		// errors it would kill the fuzz test straight away e.g. on the first invalid
		// utf-8 char
		scanner := scanner.New("fuzz", []byte(src), nil)

		for tok := range scanner.All() {
			// Property: Positions must be positive integers
			test.True(t, tok.Start >= 0, test.Context("token start position (%d) was negative", tok.Start))
			test.True(t, tok.End >= 0, test.Context("token end position (%d) was negative", tok.End))

			// Property: The kind must be one of the known kinds
			test.True(
				t,
				(tok.Kind >= token.EOF) && (tok.Kind <= token.NoRedirect),
				test.Context("token %s was not one of the pre-defined kinds", tok),
			)

			// Property: End must be >= Start
			test.True(t, tok.End >= tok.Start, test.Context("token %s had invalid start and end positions", tok))
		}
	})
}

func BenchmarkScanner(b *testing.B) {
	file := filepath.Join("testdata", "valid", "full.txtar")
	archive, err := txtar.ParseFile(file)
	test.Ok(b, err)

	src, ok := archive.Read("src.http")
	test.True(b, ok, test.Context("src.http not in %s", file))

	for b.Loop() {
		scanner := scanner.New("bench", []byte(src), testFailHandler(b))

		for {
			tok := scanner.Scan()
			if tok.Is(token.EOF, token.Error) {
				break
			}
		}
	}
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
