package spec_test

import (
	"encoding/json"
	"flag"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"go.followtheprocess.codes/req/internal/spec"
	"go.followtheprocess.codes/req/internal/syntax"
	"go.followtheprocess.codes/snapshot"
	"go.followtheprocess.codes/test"
	"go.followtheprocess.codes/txtar"
)

var (
	update = flag.Bool("update", false, "Update snapshots")
	clean  = flag.Bool("clean", false, "Clean all snapshots and recreate")
)

func TestResolve(t *testing.T) {
	test.ColorEnabled(true)

	pattern := filepath.Join("testdata", "TestResolve", "*.txtar")
	files, err := filepath.Glob(pattern)
	test.Ok(t, err)

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			archive, err := txtar.ParseFile(file)
			test.Ok(t, err)

			raw, ok := archive.Read("raw.json")
			test.True(t, ok, test.Context("archive missing raw.json"))

			want, ok := archive.Read("resolved.json")
			test.True(t, ok, test.Context("archive missing resolved.json"))

			// Unmarshal the "raw" json into a syntax.File, resolve it into a spec.File
			// then marshal that to json and it should match "resolved"
			var in syntax.File
			test.Ok(t, json.Unmarshal([]byte(raw), &in))

			resolved, err := spec.ResolveFile(in)
			test.Ok(t, err)

			got, err := json.MarshalIndent(resolved, "", "  ")
			test.Ok(t, err)

			// MarshalIndent does not add a newline
			got = append(got, '\n')

			if *update {
				err := archive.Write("resolved.json", string(got))
				test.Ok(t, err)

				err = txtar.DumpFile(file, archive)
				test.Ok(t, err)

				return
			}

			test.Diff(t, string(got), want)
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name string    // Name of the test case
		file spec.File // File under test
	}{
		{
			name: "empty",
			file: spec.File{},
		},
		{
			name: "name only",
			file: spec.File{
				Name: "FileyMcFileFace",
			},
		},
		{
			name: "name and vars",
			file: spec.File{
				Name: "SomeVars",
				Vars: map[string]string{
					"base":  "https://url.com/api/v1",
					"hello": "world",
				},
			},
		},
		{
			name: "non default timeouts",
			file: spec.File{
				Name:              "Timeouts",
				Timeout:           42 * time.Second,
				ConnectionTimeout: 12 * time.Second,
			},
		},
		{
			name: "no redirect",
			file: spec.File{
				Name:       "NoRedirect",
				NoRedirect: true,
			},
		},
		{
			name: "global prompts",
			file: spec.File{
				Name: "PromptMe",
				Prompts: []spec.Prompt{
					{Name: "value", Description: "Give me a value!"},
				},
			},
		},
		{
			name: "with simple request",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name:    "GetItem",
						Comment: "A simple request",
						Method:  http.MethodGet,
						URL:     "https://api.com/v1/items/123",
					},
				},
			},
		},
		{
			name: "request with variables",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name: "GetItem",
						Vars: map[string]string{
							"test": "yes",
						},
						Comment: "A simple request",
						Method:  http.MethodGet,
						URL:     "https://api.com/v1/items/123",
					},
				},
			},
		},
		{
			name: "with http version",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name:        "GetItem",
						Comment:     "A simple request",
						Method:      http.MethodGet,
						HTTPVersion: "HTTP/1.2",
						URL:         "https://api.com/v1/items/123",
					},
				},
			},
		},
		{
			name: "request headers",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name:   "Another Request",
						Method: http.MethodPost,
						URL:    "https://api.com/v1/items/123",
						Headers: map[string]string{
							"Accept":        "application/json",
							"Content-Type":  "application/json",
							"Authorization": "Bearer xxxxx",
						},
					},
				},
			},
		},
		{
			name: "request with timeouts",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name:              "Another Request",
						Method:            http.MethodPost,
						URL:               "https://api.com/v1/items/123",
						Timeout:           3 * time.Second,
						ConnectionTimeout: 500 * time.Millisecond,
						NoRedirect:        true,
					},
				},
			},
		},
		{
			name: "request with body file",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name:     "Another Request",
						Method:   http.MethodPost,
						URL:      "https://api.com/v1/items/123",
						BodyFile: "./body.json",
					},
				},
			},
		},
		{
			name: "request with body",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name:   "Another Request",
						Method: http.MethodPost,
						URL:    "https://api.com/v1/items/123",
						Body:   []byte(`{"some": "json", "here": "yes"}`),
					},
				},
			},
		},
		{
			name: "request with response ref",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name:         "Another Request",
						Method:       http.MethodPost,
						URL:          "https://api.com/v1/items/123",
						ResponseFile: "./response.json",
					},
				},
			},
		},
		{
			name: "request with prompt",
			file: spec.File{
				Name: "Requests",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Requests: []spec.Request{
					{
						Name: "Another",
						Prompts: []spec.Prompt{
							{Name: "value", Description: "Give me a value!"},
						},
						Method:       http.MethodPost,
						URL:          "https://api.com/v1/items/123",
						ResponseFile: "./response.json",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := snapshot.New(t, snapshot.Update(*update), snapshot.Clean(*clean))
			snap.Snap(tt.file.String())
		})
	}
}
