package req_test

import (
	"bytes"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FollowTheProcess/req/internal/req"
	"github.com/FollowTheProcess/snapshot"
	"github.com/FollowTheProcess/test"
)

var (
	update = flag.Bool("update", false, "Update snapshots")
	clean  = flag.Bool("clean", false, "Clean all snapshots and recreate")
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
		test.True(
			t,
			strings.Contains(
				got,
				`testdata/check/bad.http:2:14-27: bad timeout value: time: invalid duration "amillionyears"`,
			),
		)

		// Stdout should be empty
		test.Equal(t, stdout.String(), "")
	})
}

func TestShow(t *testing.T) {
	tests := []struct {
		name    string          // Name of the test case
		options req.ShowOptions // Options to pass simulating CLI flags
	}{
		{
			name:    "default",
			options: req.ShowOptions{},
		},
		{
			name:    "json",
			options: req.ShowOptions{JSON: true},
		},
		{
			name:    "resolved",
			options: req.ShowOptions{Resolve: true},
		},
		{
			name:    "resolved json",
			options: req.ShowOptions{Resolve: true, JSON: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := snapshot.New(t, snapshot.Clean(*clean), snapshot.Update(*update))

			file := filepath.Join("testdata", "show", "full.http")

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			r := req.New(stdout, stderr)

			err := r.Show(file, tt.options)
			test.Ok(t, err)

			test.Equal(t, stderr.String(), "")
			snap.Snap(stdout.String())
		})
	}
}
