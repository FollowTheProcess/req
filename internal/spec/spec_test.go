package spec_test

import (
	"encoding/json"
	"flag"
	"net/http"
	"testing"
	"time"

	"go.followtheprocess.codes/req/internal/spec"
	"go.followtheprocess.codes/req/internal/syntax"
	"go.followtheprocess.codes/snapshot"
	"go.followtheprocess.codes/test"
)

var (
	update = flag.Bool("update", false, "Update snapshots")
	clean  = flag.Bool("clean", false, "Clean all snapshots and recreate")
)

func TestResolve(t *testing.T) {
	test.ColorEnabled(true) // Force colour in the diffs

	tests := []struct {
		name    string      // Name of the test case
		errMsg  string      // If we wanted an error, what should it say
		in      syntax.File // Raw file in
		want    spec.File   // Expected resolved file
		wantErr bool        // Whether we want an error
	}{
		{
			name: "empty",
			in:   syntax.File{},
			want: spec.File{
				Timeout:           spec.DefaultTimeout,
				ConnectionTimeout: spec.DefaultConnectionTimeout,
			},
			wantErr: false,
			errMsg:  "",
		},
		{
			name: "globals",
			in: syntax.File{
				Name: "globals",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
			},
			want: spec.File{
				Name: "globals",
				Vars: map[string]string{
					"base": "https://api.com/v1",
				},
				Timeout:           spec.DefaultTimeout,
				ConnectionTimeout: spec.DefaultConnectionTimeout,
			},
			wantErr: false,
			errMsg:  "",
		},
		{
			name: "global prompts",
			in: syntax.File{
				Name: "globals",
				Prompts: []syntax.Prompt{
					{Name: "value", Description: "Give me a value"},
				},
			},
			want: spec.File{
				Name: "globals",
				Prompts: []spec.Prompt{
					{Name: "value", Description: "Give me a value"},
				},
				Timeout:           spec.DefaultTimeout,
				ConnectionTimeout: spec.DefaultConnectionTimeout,
			},
			wantErr: false,
			errMsg:  "",
		},
		{
			name: "single request",
			in: syntax.File{
				Name: "test.http",
				Requests: []syntax.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Name:   "#1",
						Method: "POST",
						URL:    "https://api.com/items/1",
						Body:   []byte(`{"message": "here"}`),
					},
				},
			},
			want: spec.File{
				Name: "test.http",
				Requests: []spec.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Name:              "#1",
						Method:            "POST",
						URL:               "https://api.com/items/1",
						Body:              []byte(`{"message": "here"}`),
						Timeout:           spec.DefaultTimeout,
						ConnectionTimeout: spec.DefaultConnectionTimeout,
					},
				},
				Timeout:           spec.DefaultTimeout,
				ConnectionTimeout: spec.DefaultConnectionTimeout,
			},
			wantErr: false,
			errMsg:  "",
		},
		{
			name: "single request using variables",
			in: syntax.File{
				Name: "test.http",
				Vars: map[string]string{
					"base": "https://api.com",
				},
				Requests: []syntax.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
							"X-User-ID":    "{{.Local.user_id}}",
						},
						Vars: map[string]string{
							"user_id": "123",
						},
						Name:   "#1",
						Method: "POST",
						URL:    "{{.Global.base}}/items/1",
						Body:   []byte(`{"message": "here", "user": "{{.Local.user_id}}"}`),
					},
				},
			},
			want: spec.File{
				Name: "test.http",
				Vars: map[string]string{
					"base": "https://api.com",
				},
				Requests: []spec.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
							"X-User-ID":    "123",
						},
						Vars: map[string]string{
							"user_id": "123",
						},
						Name:              "#1",
						Method:            "POST",
						URL:               "https://api.com/items/1",
						Body:              []byte(`{"message": "here", "user": "123"}`),
						Timeout:           spec.DefaultTimeout,
						ConnectionTimeout: spec.DefaultConnectionTimeout,
					},
				},
				Timeout:           spec.DefaultTimeout,
				ConnectionTimeout: spec.DefaultConnectionTimeout,
			},
			wantErr: false,
			errMsg:  "",
		},
		{
			name: "single request with prompt",
			in: syntax.File{
				Name: "test.http",
				Requests: []syntax.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Prompts: []syntax.Prompt{
							{Name: "value", Description: "Give me a value"},
						},
						Name:   "#1",
						Method: "POST",
						URL:    "https://api.com/items/1",
						Body:   []byte(`{"message": "here", "user": 123}`),
					},
				},
			},
			want: spec.File{
				Name: "test.http",
				Requests: []spec.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Prompts: []spec.Prompt{
							{Name: "value", Description: "Give me a value"},
						},
						Name:              "#1",
						Method:            "POST",
						URL:               "https://api.com/items/1",
						Body:              []byte(`{"message": "here", "user": 123}`),
						Timeout:           spec.DefaultTimeout,
						ConnectionTimeout: spec.DefaultConnectionTimeout,
					},
				},
				Timeout:           spec.DefaultTimeout,
				ConnectionTimeout: spec.DefaultConnectionTimeout,
			},
			wantErr: false,
			errMsg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := spec.ResolveFile(tt.in)
			test.WantErr(t, err, tt.wantErr)

			if err != nil {
				test.Equal(t, err.Error(), tt.errMsg)
			}

			gotJSON, err := json.MarshalIndent(got, "", "  ")
			test.Ok(t, err)

			wantJSON, err := json.MarshalIndent(tt.want, "", "  ")
			test.Ok(t, err)

			// MarhalIndent does not add a final newline
			gotJSON = append(gotJSON, '\n')
			wantJSON = append(wantJSON, '\n')

			test.DiffBytes(t, gotJSON, wantJSON)
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
