package spec_test

import (
	"encoding/json"
	"testing"

	"github.com/FollowTheProcess/req/internal/spec"
	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/test"
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
			name: "globals with interpolation",
			in: syntax.File{
				Name: "globals",
				Vars: map[string]string{
					"base": "https://api.com/v1",
					"auth": "{{base}}/auth",
				},
			},
			want: spec.File{
				Name: "globals",
				Vars: map[string]string{
					"base": "https://api.com/v1",
					"auth": "https://api.com/v1/auth",
				},
				Timeout:           spec.DefaultTimeout,
				ConnectionTimeout: spec.DefaultConnectionTimeout,
			},
			wantErr: false,
			errMsg:  "",
		},
		{
			name: "globals with undeclared variable",
			in: syntax.File{
				Name: "globals",
				Vars: map[string]string{
					"base":  "https://api.com/v1",
					"auth":  "{{base}}/auth",
					"wrong": "{{missing}}/variable",
				},
			},
			want:    spec.File{},
			wantErr: true,
			errMsg:  `use of undeclared variable "{{missing}}" in interpolation`,
		},
		{
			name: "globals with unterminated interpolation",
			in: syntax.File{
				Name: "globals",
				Vars: map[string]string{
					"base": "https://api.com/v1",
					"auth": "{{base/auth",
				},
			},
			want:    spec.File{},
			wantErr: true,
			errMsg:  `unterminated variable interpolation: "{{base/aut"`,
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
			name: "single request using globals",
			in: syntax.File{
				Name: "test.http",
				Vars: map[string]string{
					"base":    "https://api.com",
					"user_id": "123",
				},
				Requests: []syntax.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
							"X-User-ID":    "{{user_id}}",
						},
						Name:   "#1",
						Method: "POST",
						URL:    "{{base}}/items/1",
						Body:   []byte(`{"message": "here", "user": "{{user_id}}"}`),
					},
				},
			},
			want: spec.File{
				Name: "test.http",
				Vars: map[string]string{
					"base":    "https://api.com",
					"user_id": "123",
				},
				Requests: []spec.Request{
					{
						Headers: map[string]string{
							"Content-Type": "application/json",
							"X-User-ID":    "123",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := spec.ResolveFile(tt.in)
			test.WantErr(t, err, tt.wantErr)

			if err != nil {
				test.Equal(t, err.Error(), tt.errMsg)
			}

			if !spec.Equal(got, tt.want) {
				// Do a nice diff using JSON
				gotJSON, err := json.MarshalIndent(got, "", "  ")
				test.Ok(t, err)

				wantJSON, err := json.MarshalIndent(tt.want, "", "  ")
				test.Ok(t, err)

				gotJSON = append(gotJSON, '\n')
				wantJSON = append(wantJSON, '\n')

				test.DiffBytes(t, gotJSON, wantJSON)
			}
		})
	}
}
