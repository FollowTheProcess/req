package req_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/FollowTheProcess/req/internal/req"
	"github.com/FollowTheProcess/test"
)

func TestCheck(t *testing.T) {
	good := filepath.Join("testdata", "check", "good.http")
	bad := filepath.Join("testdata", "check", "bad.http")

	t.Run("good", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		req := req.New(stdout, stderr)

		err := req.Check(good)
		test.Ok(t, err)

		// Stderr should be empty
		test.Equal(t, stderr.String(), "")

		// Stdout should have the success message
		test.Equal(t, stdout.String(), "Success: testdata/check/good.http is valid\n")
	})

	t.Run("bad", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		req := req.New(stdout, stderr)

		err := req.Check(bad)
		test.Err(t, err)

		// Stderr should have the syntax error
		test.Equal(
			t,
			stderr.String(),
			`testdata/check/bad.http:2:14-27: bad timeout value: time: invalid duration "amillionyears"`+"\n",
		)

		// Stdout should be empty
		test.Equal(t, stdout.String(), "")
	})
}
