package scanner_test

import (
	"flag"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/scanner"
	"github.com/FollowTheProcess/req/internal/syntax/token"
	"github.com/FollowTheProcess/test"
	"github.com/FollowTheProcess/txtar"
)

var update = flag.Bool("update", false, "Update snapshots and testdata")

func TestScanBasics(t *testing.T) {
	tests := []struct {
		name string        // Name of the test case
		src  string        // Source text to scan
		want []token.Token // Expected tokens
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
			src:  "# A comment",
			want: []token.Token{
				{Kind: token.Comment, Start: 2, End: 11},
				{Kind: token.EOF, Start: 11, End: 11},
			},
		},
		{
			name: "slash comment",
			src:  "// A comment",
			want: []token.Token{
				{Kind: token.Comment, Start: 3, End: 12},
				{Kind: token.EOF, Start: 12, End: 12},
			},
		},
		{
			name: "hash comment with line tail",
			src:  "# A comment\n",
			want: []token.Token{
				{Kind: token.Comment, Start: 2, End: 11},
				// There's a "hidden" (ignored) newline here, hence why EOF starts at 12
				{Kind: token.EOF, Start: 12, End: 12},
			},
		},
		{
			name: "slash comment with line tail",
			src:  "// A comment\n",
			want: []token.Token{
				{Kind: token.Comment, Start: 3, End: 12},
				// There's a "hidden" (ignored) newline here, hence why EOF starts at 13
				{Kind: token.EOF, Start: 13, End: 13},
			},
		},
		{
			name: "text",
			src:  "sometext",
			want: []token.Token{
				{Kind: token.Text, Start: 0, End: 8},
				{Kind: token.EOF, Start: 8, End: 8},
			},
		},
		{
			name: "method but lowercase",
			src:  "post",
			want: []token.Token{
				{Kind: token.Text, Start: 0, End: 4},
				{Kind: token.EOF, Start: 4, End: 4},
			},
		},
		{
			name: "method get",
			src:  "GET",
			want: []token.Token{
				{Kind: token.MethodGet, Start: 0, End: 3},
				{Kind: token.EOF, Start: 3, End: 3},
			},
		},
		{
			name: "method head",
			src:  "HEAD",
			want: []token.Token{
				{Kind: token.MethodHead, Start: 0, End: 4},
				{Kind: token.EOF, Start: 4, End: 4},
			},
		},
		{
			name: "method post",
			src:  "POST",
			want: []token.Token{
				{Kind: token.MethodPost, Start: 0, End: 4},
				{Kind: token.EOF, Start: 4, End: 4},
			},
		},
		{
			name: "method put",
			src:  "PUT",
			want: []token.Token{
				{Kind: token.MethodPut, Start: 0, End: 3},
				{Kind: token.EOF, Start: 3, End: 3},
			},
		},
		{
			name: "method delete",
			src:  "DELETE",
			want: []token.Token{
				{Kind: token.MethodDelete, Start: 0, End: 6},
				{Kind: token.EOF, Start: 6, End: 6},
			},
		},
		{
			name: "method connect",
			src:  "CONNECT",
			want: []token.Token{
				{Kind: token.MethodConnect, Start: 0, End: 7},
				{Kind: token.EOF, Start: 7, End: 7},
			},
		},
		{
			name: "method patch",
			src:  "PATCH",
			want: []token.Token{
				{Kind: token.MethodPatch, Start: 0, End: 5},
				{Kind: token.EOF, Start: 5, End: 5},
			},
		},
		{
			name: "method options",
			src:  "OPTIONS",
			want: []token.Token{
				{Kind: token.MethodOptions, Start: 0, End: 7},
				{Kind: token.EOF, Start: 7, End: 7},
			},
		},
		{
			name: "method trace",
			src:  "TRACE",
			want: []token.Token{
				{Kind: token.MethodTrace, Start: 0, End: 5},
				{Kind: token.EOF, Start: 5, End: 5},
			},
		},
		{
			name: "integer",
			src:  "256",
			want: []token.Token{
				{Kind: token.Number, Start: 0, End: 3},
				{Kind: token.EOF, Start: 3, End: 3},
			},
		},
		{
			name: "float",
			src:  "3.14159",
			want: []token.Token{
				{Kind: token.Number, Start: 0, End: 7},
				{Kind: token.EOF, Start: 7, End: 7},
			},
		},
		{
			name: "request sep",
			src:  "###",
			want: []token.Token{
				{Kind: token.RequestSeparator, Start: 0, End: 3},
				{Kind: token.EOF, Start: 3, End: 3},
			},
		},
		{
			name: "request sep with line tail",
			src:  "###\n",
			want: []token.Token{
				{Kind: token.RequestSeparator, Start: 0, End: 3},
				{Kind: token.EOF, Start: 4, End: 4},
			},
		},
		{
			name: "request sep with name",
			src:  "### My Request",
			want: []token.Token{
				{Kind: token.RequestSeparator, Start: 0, End: 3},
				{Kind: token.Text, Start: 4, End: 14}, // <- The name
				{Kind: token.EOF, Start: 14, End: 14},
			},
		},
		{
			name: "request sep with name and line tail",
			src:  "### My Request\n",
			want: []token.Token{
				{Kind: token.RequestSeparator, Start: 0, End: 3},
				{Kind: token.Text, Start: 4, End: 14}, // <- The name
				// Ignored '\n' so no token but position increases by 1
				{Kind: token.EOF, Start: 15, End: 15},
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
			name: "eq",
			src:  "=",
			want: []token.Token{
				{Kind: token.Eq, Start: 0, End: 1},
				{Kind: token.EOF, Start: 1, End: 1},
			},
		},
		{
			name: "colon",
			src:  ":",
			want: []token.Token{
				{Kind: token.Colon, Start: 0, End: 1},
				{Kind: token.EOF, Start: 1, End: 1},
			},
		},
		{
			name: "at with ident",
			src:  "@something",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.EOF, Start: 10, End: 10},
			},
		},
		{
			name: "at with ident and line tail",
			src:  "@something\n",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.EOF, Start: 11, End: 11},
			},
		},
		{
			name: "at ident equal value",
			src:  "@something=value",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.Eq, Start: 10, End: 11},
				{Kind: token.Text, Start: 11, End: 16},
				{Kind: token.EOF, Start: 16, End: 16},
			},
		},
		{
			name: "at ident equal integer",
			src:  "@timeout=20",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 8},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Number, Start: 9, End: 11},
				{Kind: token.EOF, Start: 11, End: 11},
			},
		},
		{
			name: "at ident equal duration",
			src:  "@timeout=20s",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 8},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Number, Start: 9, End: 11},
				{Kind: token.Text, Start: 11, End: 12},
				{Kind: token.EOF, Start: 12, End: 12},
			},
		},
		{
			name: "at ident equal space duration",
			src:  "@timeout=20 s",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 8},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Number, Start: 9, End: 11},
				{Kind: token.Text, Start: 12, End: 13},
				{Kind: token.EOF, Start: 13, End: 13},
			},
		},
		{
			name: "at ident equal duration line tail",
			src:  "@timeout=20s\n",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 8},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Number, Start: 9, End: 11},
				{Kind: token.Text, Start: 11, End: 12},
				{Kind: token.EOF, Start: 13, End: 13},
			},
		},
		{
			name: "at ident equal value line tail",
			src:  "@something=value\n",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.Eq, Start: 10, End: 11},
				{Kind: token.Text, Start: 11, End: 16},
				{Kind: token.EOF, Start: 17, End: 17},
			},
		},
		{
			name: "at ident space value",
			src:  "@something value",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.Text, Start: 11, End: 16},
				{Kind: token.EOF, Start: 16, End: 16},
			},
		},
		{
			name: "at ident space value line tail",
			src:  "@something value\n",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.Text, Start: 11, End: 16},
				{Kind: token.EOF, Start: 17, End: 17},
			},
		},
		{
			name: "at ident space equal value",
			src:  "@something = value",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.Eq, Start: 11, End: 12},
				{Kind: token.Text, Start: 13, End: 18},
				{Kind: token.EOF, Start: 18, End: 18},
			},
		},
		{
			name: "at ident space equal value line tail",
			src:  "@something = value\n",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Ident, Start: 1, End: 10},
				{Kind: token.Eq, Start: 11, End: 12},
				{Kind: token.Text, Start: 13, End: 18},
				{Kind: token.EOF, Start: 19, End: 19},
			},
		},
		{
			name: "http version",
			src:  "HTTP/1.1",
			want: []token.Token{
				{Kind: token.HTTPVersion, Start: 0, End: 8},
				{Kind: token.EOF, Start: 8, End: 8},
			},
		},
		{
			name: "http version two",
			src:  "HTTP/2",
			want: []token.Token{
				{Kind: token.HTTPVersion, Start: 0, End: 6},
				{Kind: token.EOF, Start: 6, End: 6},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.src)
			scanner, err := scanner.New(tt.name, r, testFailHandler(t))
			test.Ok(t, err)

			var tokens []token.Token
			for {
				tok := scanner.Scan()
				tokens = append(tokens, tok)
				if tok.Kind == token.EOF {
					break
				}
			}

			test.EqualFunc(t, tokens, tt.want, slices.Equal, test.Context("token stream mismatch"))
		})
	}
}

func TestValid(t *testing.T) {
	pattern := filepath.Join("testdata", "valid", "*.txtar")
	files, err := filepath.Glob(pattern)
	test.Ok(t, err)

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			archive, err := txtar.ParseFile(file)
			test.Ok(t, err)

			src, ok := archive.Read("src.http")
			test.True(t, ok, test.Context("archive missing src.http"))

			want, ok := archive.Read("tokens.txt")
			test.True(t, ok, test.Context("archive missing tokens.txt"))

			scanner, err := scanner.New(name, strings.NewReader(src), testFailHandler(t))
			test.Ok(t, err)

			var tokens []token.Token
			for {
				tok := scanner.Scan()
				tokens = append(tokens, tok)
				if tok.Kind == token.EOF {
					break
				}
			}

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
		scanner, err := scanner.New("fuzz", strings.NewReader(src), nil)
		test.Ok(t, err)

		for {
			tok := scanner.Scan()

			// Property: End must be >= Start
			test.True(t, tok.End >= tok.Start)

			if tok.Kind == token.EOF || tok.Kind == token.Error {
				break
			}
		}
	})
}

// testFailHandler returns a [scanner.ErrorHandler] that handles scanning errors by failing
// the enclosing test.
func testFailHandler(tb testing.TB) syntax.ErrorHandler {
	tb.Helper()
	return func(pos syntax.Position, msg string) {
		tb.Fatalf("%s: %s", pos, msg)
	}
}
