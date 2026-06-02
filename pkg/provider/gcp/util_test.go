package gcp

import (
	"testing"
)

// TestSanitizeImageName is superseded by pkg/util.TestSanitizeBucketName —
// GCP image/bucket names share the same constraints (lowercase, hyphens ok, max 63).

func TestValidateShareProjectIds(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		wantErr bool
	}{
		{"empty", []string{}, false},
		{"single", []string{"571214177986"}, false},
		{"distinct", []string{"571214177986", "123456789012"}, false},
		{"duplicate", []string{"571214177986", "571214177986"}, true},
		{"duplicate among many", []string{"111", "222", "111", "333"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateShareProjectIds(tt.ids)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateShareProjectIds(%v) error = %v, wantErr %v", tt.ids, err, tt.wantErr)
			}
		})
	}
}
