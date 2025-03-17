package req_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
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
		want := fmt.Sprintf("Success: %s is valid\n", good)
		test.Equal(t, stdout.String(), want)
	})

	t.Run("bad", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		req := req.New(stdout, stderr)

		err := req.Check(bad)
		test.Err(t, err)

		got := stderr.String()

		// Replace \ with / on windows
		if runtime.GOOS == "windows" {
			got = strings.ReplaceAll(got, `\`, "/")
		}

		// Stderr should have the syntax error
		test.True(t, strings.Contains(got, `testdata/check/bad.http:2:14-27: bad timeout value: time: invalid duration "amillionyears"`))

		// Stdout should be empty
		test.Equal(t, stdout.String(), "")
	})
}
