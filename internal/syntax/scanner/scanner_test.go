package scanner_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/FollowTheProcess/req/internal/syntax/scanner"
	"github.com/FollowTheProcess/req/internal/syntax/token"
	"github.com/FollowTheProcess/test"
)

func TestScanner(t *testing.T) {
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
			name: "bom",
			src:  "\ufeff",
			want: []token.Token{
				{Kind: token.EOF, Start: 3, End: 3},
			},
		},
		{
			name: "hash",
			src:  "#",
			want: []token.Token{
				{Kind: token.Hash, Start: 0, End: 1},
				{Kind: token.EOF, Start: 1, End: 1},
			},
		},
		{
			name: "slash",
			src:  "/",
			want: []token.Token{
				{Kind: token.Slash, Start: 0, End: 1},
				{Kind: token.EOF, Start: 1, End: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.src)
			scanner, err := scanner.New(tt.name, r)
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
