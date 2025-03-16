package spec_test

import (
	"encoding/json"
	"testing"

	"github.com/FollowTheProcess/req/internal/spec"
	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/test"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name    string      // Name of the test case
		errMsg  string      // If we wanted an error, what should it say
		in      syntax.File // Raw file in
		want    spec.File   // Expected resolved file
		wantErr bool        // Whether we want an error
	}{
		{
			name:    "empty",
			in:      syntax.File{},
			want:    spec.File{Timeout: spec.DefaultTimeout, ConnectionTimeout: spec.DefaultConnectionTimeout},
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

			if !spec.FileEqual(got, tt.want) {
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
