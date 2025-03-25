package req_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

		app := req.New(stdout, stderr, false)

		err := app.Check([]string{good}, req.CheckOptions{})
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

		app := req.New(stdout, stderr, false)

		err := app.Check([]string{bad}, req.CheckOptions{})
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
	good := filepath.Join("testdata", "check", "good.http")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	app := req.New(stdout, stderr, false)

	err := app.Show(good, req.ShowOptions{})
	test.Ok(t, err)

	test.Equal(t, stderr.String(), "")
	test.True(t, strings.Contains(stdout.String(), "### Body"))
}

func TestDo(t *testing.T) {
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"stuff": "here"}`)
	}

	server := httptest.NewServer(http.HandlerFunc(testHandler))
	defer server.Close()

	httpFile := fmt.Sprintf(`### Test
GET %s
Accept: application/json
`, server.URL)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	app := req.New(stdout, stderr, false)

	file, err := os.CreateTemp(t.TempDir(), "test*.http")
	test.Ok(t, err)

	_, err = file.WriteString(httpFile)
	test.Ok(t, err)
	test.Ok(t, file.Close())

	options := req.DoOptions{}

	err = app.Do(file.Name(), "Test", options)
	test.Ok(t, err)

	test.Equal(t, stdout.String(), "200 OK\n{\"stuff\": \"here\"}\n")
}
