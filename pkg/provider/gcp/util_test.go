package gcp

import (
	"strings"
	"testing"
)

func TestSanitizeImageName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "MyImage", "myimage"},
		{"underscores to hyphens", "my_image_name", "my-image-name"},
		{"dots to hyphens", "image.v1.2.3", "image-v1-2-3"},
		{"mixed", "RHEL_AI.v1_2", "rhel-ai-v1-2"},
		{"truncate at 63", strings.Repeat("a", 70), strings.Repeat("a", 63)},
		{"trim trailing hyphens after truncation", strings.Repeat("a", 62) + "._", strings.Repeat("a", 62)},
		{"already valid", "my-image-123", "my-image-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeImageName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeImageName(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) > 63 {
				t.Errorf("sanitizeImageName(%q) length %d exceeds 63", tt.input, len(got))
			}
			if strings.HasSuffix(got, "-") {
				t.Errorf("sanitizeImageName(%q) = %q: has trailing hyphen", tt.input, got)
			}
		})
	}
}
