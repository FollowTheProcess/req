package token_test

import (
	"testing"

	"github.com/FollowTheProcess/req/internal/syntax/token"
	"github.com/FollowTheProcess/test"
)

func TestString(t *testing.T) {
	tests := []struct {
		want string      // Expected string
		tok  token.Token // The token under test
	}{
		{
			tok:  token.Token{Kind: token.EOF, Start: 0, End: 0},
			want: "<Token::EOF start=0, end=0>",
		},
		{
			tok:  token.Token{Kind: token.Error, Start: 1, End: 12},
			want: "<Token::Error start=1, end=12>",
		},
		{
			tok:  token.Token{Kind: token.Hash, Start: 4, End: 5},
			want: "<Token::Hash start=4, end=5>",
		},
		{
			tok:  token.Token{Kind: token.Slash, Start: 26, End: 27},
			want: "<Token::Slash start=26, end=27>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.tok.Kind.String(), func(t *testing.T) {
			test.Equal(t, tt.tok.String(), tt.want)
		})
	}
}
