package util

import (
	"strings"
	"testing"
)

func TestSanitizeStorageAccountName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "MyStorage", "mystorage"},
		{"strips hyphens", "my-storage", "mystorage"},
		{"strips dots", "my.storage.v1", "mystoragev1"},
		{"strips underscores", "my_storage", "mystorage"},
		{"strips special chars", "my storage!@#", "mystorage"},
		{"truncate at 24", strings.Repeat("a", 30), strings.Repeat("a", 24)},
		{"already valid", "mystorage123", "mystorage123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeStorageAccountName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeStorageAccountName(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) > 24 {
				t.Errorf("SanitizeStorageAccountName(%q) length %d exceeds 24", tt.input, len(got))
			}
			if strings.ContainsAny(got, "-_.") {
				t.Errorf("SanitizeStorageAccountName(%q) = %q: contains invalid character", tt.input, got)
			}
		})
	}
}

func TestSanitizeBucketName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "MyBucket", "mybucket"},
		{"underscores to hyphens", "my_bucket_name", "my-bucket-name"},
		{"dots to hyphens", "bucket.v1.2.3", "bucket-v1-2-3"},
		{"mixed", "RHEL_AI.v1_2", "rhel-ai-v1-2"},
		{"strips special chars", "my bucket!@#", "mybucket"},
		{"strips spaces", "my image v1", "myimagev1"},
		{"truncate at 63", strings.Repeat("a", 70), strings.Repeat("a", 63)},
		{"trim trailing hyphens after truncation", strings.Repeat("a", 62) + "._", strings.Repeat("a", 62)},
		{"already valid", "my-bucket-123", "my-bucket-123"},
		{"keeps hyphens", "my-image-name", "my-image-name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeBucketName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeBucketName(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) > 63 {
				t.Errorf("SanitizeBucketName(%q) length %d exceeds 63", tt.input, len(got))
			}
			if strings.HasSuffix(got, "-") {
				t.Errorf("SanitizeBucketName(%q) = %q: has trailing hyphen", tt.input, got)
			}
		})
	}
}
