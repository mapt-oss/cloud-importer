package aws

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

func TestStableBucketName(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		wantStart string
	}{
		{"adds ci- prefix", "rhelai-1.4-x86_64", "ci-rhelai-1-4-x86-64"},
		{"same input same output", "rhelai-1.4", "ci-rhelai-1-4"},
		{"uppercase lowercased", "RHELAI", "ci-rhelai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stableBucketName(tt.imageName)
			if *got != tt.wantStart {
				t.Errorf("stableBucketName(%q) = %q, want %q", tt.imageName, *got, tt.wantStart)
			}
			if len(*got) > 63 {
				t.Errorf("stableBucketName(%q) length %d exceeds 63", tt.imageName, len(*got))
			}
			if strings.HasSuffix(*got, "-") {
				t.Errorf("stableBucketName(%q) = %q: has trailing hyphen", tt.imageName, *got)
			}
		})
	}

	t.Run("deterministic on repeated calls", func(t *testing.T) {
		name := "rhelai-1.4-1-x86_64"
		first := stableBucketName(name)
		second := stableBucketName(name)
		if *first != *second {
			t.Errorf("stableBucketName(%q) not deterministic: %q vs %q", name, *first, *second)
		}
	})
}
