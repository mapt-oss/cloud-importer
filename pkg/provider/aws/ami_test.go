package aws

import (
	"testing"
)

func TestOrgId(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		expected string
	}{
		{
			name:     "org-level ARN returns org ID",
			arn:      "arn:aws:organizations::329260820478:organization/o-melcpnc7lj",
			expected: "o-melcpnc7lj",
		},
		{
			name:     "account-level ARN returns org-account compound ID",
			arn:      "arn:aws:organizations::329260820478:account/o-melcpnc7lj/851725220677",
			expected: "o-melcpnc7lj-851725220677",
		},
		{
			name:     "second account-level ARN from same org returns different ID",
			arn:      "arn:aws:organizations::329260820478:account/o-melcpnc7lj/585132637328",
			expected: "o-melcpnc7lj-585132637328",
		},
		{
			name:     "OU-level ARN returns ou compound ID",
			arn:      "arn:aws:organizations::329260820478:ou/o-melcpnc7lj/ou-xxxx-yyyyyyyy",
			expected: "o-melcpnc7lj-ou-xxxx-yyyyyyyy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orgId(&tt.arn)
			if got != tt.expected {
				t.Errorf("orgId(%q) = %q, want %q", tt.arn, got, tt.expected)
			}
		})
	}
}

// TestOrgIdUniqueness verifies that two account ARNs from the same org produce
// distinct resource name suffixes (the root cause of issue #82).
func TestOrgIdUniqueness(t *testing.T) {
	arn1 := "arn:aws:organizations::329260820478:account/o-melcpnc7lj/851725220677"
	arn2 := "arn:aws:organizations::329260820478:account/o-melcpnc7lj/585132637328"

	id1 := orgId(&arn1)
	id2 := orgId(&arn2)

	if id1 == id2 {
		t.Errorf("two account ARNs from the same org produced the same ID %q — would cause duplicate Pulumi URN", id1)
	}
}
