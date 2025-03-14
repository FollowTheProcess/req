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
	"go.uber.org/goleak"
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
			name: "hash comment request name",
			src:  "# @name Test",
			want: []token.Token{
				{Kind: token.At, Start: 2, End: 3},
				{Kind: token.Ident, Start: 3, End: 7},
				{Kind: token.Text, Start: 8, End: 12},
				{Kind: token.EOF, Start: 11, End: 11},
			},
		},
		{
			name: "hash comment request equals name",
			src:  "# @name = Test",
			want: []token.Token{
				{Kind: token.At, Start: 2, End: 3},
				{Kind: token.Ident, Start: 3, End: 7},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Text, Start: 10, End: 13},
				{Kind: token.EOF, Start: 13, End: 13},
			},
		},
		{
			name: "hash comment request equals name with line tail",
			src:  "# @name = Test\n",
			want: []token.Token{
				{Kind: token.At, Start: 2, End: 3},
				{Kind: token.Ident, Start: 3, End: 7},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Text, Start: 10, End: 13},
				{Kind: token.EOF, Start: 14, End: 14},
			},
		},
		{
			name: "slash comment request name",
			src:  "// @name Test",
			want: []token.Token{
				{Kind: token.At, Start: 3, End: 4},
				{Kind: token.Ident, Start: 4, End: 8},
				{Kind: token.Text, Start: 9, End: 13},
				{Kind: token.EOF, Start: 13, End: 13},
			},
		},
		{
			name: "slash comment request equals name",
			src:  "// @name = Test",
			want: []token.Token{
				{Kind: token.At, Start: 3, End: 4},
				{Kind: token.Ident, Start: 4, End: 8},
				{Kind: token.Eq, Start: 9, End: 10},
				{Kind: token.Text, Start: 11, End: 15},
				{Kind: token.EOF, Start: 15, End: 15},
			},
		},
		{
			name: "slash comment request equals name with line tail",
			src:  "// @name = Test\n",
			want: []token.Token{
				{Kind: token.At, Start: 3, End: 4},
				{Kind: token.Ident, Start: 4, End: 8},
				{Kind: token.Eq, Start: 9, End: 10},
				{Kind: token.Text, Start: 11, End: 15},
				{Kind: token.EOF, Start: 16, End: 16},
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
			src:  "GET https://api.github.com/repos",
			want: []token.Token{
				{Kind: token.MethodGet, Start: 0, End: 3},
				{Kind: token.URL, Start: 4, End: 32},
				{Kind: token.EOF, Start: 32, End: 32},
			},
		},
		{
			name: "method head",
			src:  "HEAD {{base}}/person/1",
			want: []token.Token{
				{Kind: token.MethodHead, Start: 0, End: 4},
				{Kind: token.URL, Start: 5, End: 22},
				{Kind: token.EOF, Start: 22, End: 22},
			},
		},
		{
			name: "method post",
			src:  "POST http://insecure.com/api/{{template}}/something",
			want: []token.Token{
				{Kind: token.MethodPost, Start: 0, End: 4},
				{Kind: token.URL, Start: 5, End: 51},
				{Kind: token.EOF, Start: 51, End: 51},
			},
		},
		{
			name: "method put",
			src:  "PUT https://api.github.com/users",
			want: []token.Token{
				{Kind: token.MethodPut, Start: 0, End: 3},
				{Kind: token.URL, Start: 4, End: 32},
				{Kind: token.EOF, Start: 32, End: 32},
			},
		},
		{
			name: "method delete",
			src:  "DELETE {{base}}/v2/auth", // Lol delete auth okay sure
			want: []token.Token{
				{Kind: token.MethodDelete, Start: 0, End: 6},
				{Kind: token.URL, Start: 7, End: 23},
				{Kind: token.EOF, Start: 23, End: 23},
			},
		},
		{
			name: "method connect",
			src:  "CONNECT https://these.com/are/hard/now",
			want: []token.Token{
				{Kind: token.MethodConnect, Start: 0, End: 7},
				{Kind: token.URL, Start: 8, End: 38},
				{Kind: token.EOF, Start: 38, End: 38},
			},
		},
		{
			name: "method patch",
			src:  "PATCH {{base}}/items/1",
			want: []token.Token{
				{Kind: token.MethodPatch, Start: 0, End: 5},
				{Kind: token.URL, Start: 6, End: 22},
				{Kind: token.EOF, Start: 22, End: 22},
			},
		},
		{
			name: "method options",
			src:  "OPTIONS {{base}}/items/1",
			want: []token.Token{
				{Kind: token.MethodOptions, Start: 0, End: 7},
				{Kind: token.URL, Start: 8, End: 24},
				{Kind: token.EOF, Start: 24, End: 24},
			},
		},
		{
			name: "method trace",
			src:  "TRACE {{base}}/orders/256",
			want: []token.Token{
				{Kind: token.MethodTrace, Start: 0, End: 5},
				{Kind: token.URL, Start: 6, End: 25},
				{Kind: token.EOF, Start: 25, End: 25},
			},
		},
		{
			name: "integer",
			src:  "256",
			want: []token.Token{
				{Kind: token.Text, Start: 0, End: 3},
				{Kind: token.EOF, Start: 3, End: 3},
			},
		},
		{
			name: "float",
			src:  "3.14159",
			want: []token.Token{
				{Kind: token.Text, Start: 0, End: 7},
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
			src:  "@timeout=20s",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Timeout, Start: 1, End: 8},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Text, Start: 9, End: 12},
				{Kind: token.EOF, Start: 12, End: 12},
			},
		},
		{
			name: "at timeout equal duration line tail",
			src:  "@timeout=20s\n",
			want: []token.Token{
				{Kind: token.At, Start: 0, End: 1},
				{Kind: token.Timeout, Start: 1, End: 8},
				{Kind: token.Eq, Start: 8, End: 9},
				{Kind: token.Text, Start: 9, End: 12},
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
			defer goleak.VerifyNone(t)
			src := []byte(tt.src)
			scanner := scanner.New(tt.name, src, testFailHandler(t))

			var tokens []token.Token
			for {
				tok := scanner.Scan()
				tokens = append(tokens, tok)
				if tok.Kind == token.EOF || tok.Kind == token.Error {
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
			defer goleak.VerifyNone(t)
			archive, err := txtar.ParseFile(file)
			test.Ok(t, err)

			src, ok := archive.Read("src.http")
			test.True(t, ok, test.Context("archive missing src.http"))

			want, ok := archive.Read("tokens.txt")
			test.True(t, ok, test.Context("archive missing tokens.txt"))

			scanner := scanner.New(name, []byte(src), testFailHandler(t))

			var tokens []token.Token
			for {
				tok := scanner.Scan()
				tokens = append(tokens, tok)
				if tok.Kind == token.EOF || tok.Kind == token.Error {
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
		scanner := scanner.New("fuzz", []byte(src), nil)

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

func BenchmarkScanner(b *testing.B) {
	file := filepath.Join("testdata", "valid", "full2.txtar")
	archive, err := txtar.ParseFile(file)
	test.Ok(b, err)

	src, ok := archive.Read("src.http")
	test.True(b, ok, test.Context("src.http not in %s", file))

	for b.Loop() {
		scanner := scanner.New("bench", []byte(src), testFailHandler(b))

		for {
			tok := scanner.Scan()
			if tok.Kind == token.EOF || tok.Kind == token.Error {
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
