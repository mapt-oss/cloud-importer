package gcp

import (
	"testing"
)

func TestParseGCSBackendURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantBucket string
		wantPrefix string
		wantErr    bool
	}{
		{
			name:       "bucket and prefix",
			input:      "gs://my-bucket/path/to/state",
			wantBucket: "my-bucket",
			wantPrefix: "path/to/state",
		},
		{
			name:       "bucket only",
			input:      "gs://my-bucket",
			wantBucket: "my-bucket",
			wantPrefix: "",
		},
		{
			name:       "bucket with trailing slash",
			input:      "gs://my-bucket/",
			wantBucket: "my-bucket",
			wantPrefix: "",
		},
		{
			name:    "wrong scheme",
			input:   "s3://my-bucket/path",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, prefix, err := parseGCSBackendURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseGCSBackendURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if bucket != tt.wantBucket {
				t.Errorf("bucket = %q, want %q", bucket, tt.wantBucket)
			}
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
		})
	}
}
