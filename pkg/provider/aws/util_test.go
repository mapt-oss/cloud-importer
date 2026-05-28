package aws

import (
	"strings"
	"testing"
)

func TestStableBucketName(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		wantStart string
	}{
		{"adds tmp- prefix", "rhelai-1.4-x86_64", "tmp-rhelai-1-4-x86-64"},
		{"same input same output", "rhelai-1.4", "tmp-rhelai-1-4"},
		{"uppercase lowercased", "RHELAI", "tmp-rhelai"},
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
