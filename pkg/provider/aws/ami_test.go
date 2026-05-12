package aws

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func TestPulumiResourceId(t *testing.T) {
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
			name:     "plain account ID returns account ID as-is",
			arn:      "851725220677",
			expected: "851725220677",
		},
		{
			name:     "OU-level ARN returns ou compound ID",
			arn:      "arn:aws:organizations::329260820478:ou/o-melcpnc7lj/ou-xxxx-yyyyyyyy",
			expected: "o-melcpnc7lj-ou-xxxx-yyyyyyyy",
		},
		{
			name:     "malformed ARN too few segments returns ARN as-is (no panic)",
			arn:      "arn:aws:organizations::329260820478",
			expected: "arn:aws:organizations::329260820478",
		},
		{
			name:     "malformed ARN resource part has no slash returns ARN as-is (no panic)",
			arn:      "arn:aws:organizations::329260820478:organization",
			expected: "arn:aws:organizations::329260820478:organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pulumiResourceId(tt.arn)
			if got != tt.expected {
				t.Errorf("orgId(%q) = %q, want %q", tt.arn, got, tt.expected)
			}
		})
	}
}

// TestOrgIdUniqueness verifies that two account ARNs from the same org produce
// distinct resource name suffixes (the root cause of issue #82).
func TestPulumiResourceIdUniqueness(t *testing.T) {
	arn1 := "arn:aws:organizations::329260820478:account/o-melcpnc7lj/851725220677"
	arn2 := "arn:aws:organizations::329260820478:account/o-melcpnc7lj/585132637328"

	id1 := pulumiResourceId(arn1)
	id2 := pulumiResourceId(arn2)

	if id1 == id2 {
		t.Errorf("two account ARNs from the same org produced the same ID %q — would cause duplicate Pulumi URN", id1)
	}
}

func TestLaunchPermArgs(t *testing.T) {
	imageId := pulumi.String("ami-12345678")

	tests := []struct {
		name        string
		arn         string
		wantOrgArn  bool
		wantAcctId  bool
		wantOUArn   bool
	}{
		{
			name:       "org ARN routes to OrganizationArn",
			arn:        "arn:aws:organizations::329260820478:organization/o-melcpnc7lj",
			wantOrgArn: true,
		},
		{
			name:       "account ARN routes to AccountId",
			arn:        "arn:aws:organizations::329260820478:account/o-melcpnc7lj/851725220677",
			wantAcctId: true,
		},
		{
			name:       "plain account ID routes to AccountId",
			arn:        "851725220677",
			wantAcctId: true,
		},
		{
			name:      "OU ARN routes to OrganizationalUnitArn",
			arn:       "arn:aws:organizations::329260820478:ou/o-melcpnc7lj/ou-xxxx-yyyyyyyy",
			wantOUArn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := launchPermArgs(imageId, tt.arn)
			if got := args.OrganizationArn != nil; got != tt.wantOrgArn {
				t.Errorf("OrganizationArn set=%v, want %v", got, tt.wantOrgArn)
			}
			if got := args.AccountId != nil; got != tt.wantAcctId {
				t.Errorf("AccountId set=%v, want %v", got, tt.wantAcctId)
			}
			if got := args.OrganizationalUnitArn != nil; got != tt.wantOUArn {
				t.Errorf("OrganizationalUnitArn set=%v, want %v", got, tt.wantOUArn)
			}
		})
	}
}

func TestPulumiResourceIdOrgLevel(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		expected string
	}{
		{
			name:     "org ARN with one management account",
			arn:      "arn:aws:organizations::329260820478:organization/o-meltunc7lj",
			expected: "o-meltunc7lj",
		},
		{
			name:     "org ARN with different management account, same org ID",
			arn:      "arn:aws:organizations::329260824578:organization/o-meltunc7lj",
			expected: "o-meltunc7lj",
		},
		{
			name:     "org ARN with different org ID",
			arn:      "arn:aws:organizations::329260820478:organization/o-different1",
			expected: "o-different1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pulumiResourceId(tt.arn)
			if got != tt.expected {
				t.Errorf("orgId(%q) = %q, want %q", tt.arn, got, tt.expected)
			}
		})
	}
}

func TestValidateShareOrgIds(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		wantErr bool
	}{
		{
			name: "reviewer case: same org ID different management accounts",
			ids: []string{
				"arn:aws:organizations::329260820478:organization/o-meltunc7lj",
				"arn:aws:organizations::329260824578:organization/o-meltunc7lj",
			},
			wantErr: true,
		},
		{
			name: "exact duplicate org ARN",
			ids: []string{
				"arn:aws:organizations::329260820478:organization/o-meltunc7lj",
				"arn:aws:organizations::329260820478:organization/o-meltunc7lj",
			},
			wantErr: true,
		},
		{
			name:    "exact duplicate plain account ID",
			ids:     []string{"851725220677", "851725220677"},
			wantErr: true,
		},
		{
			name: "two org ARNs with different org IDs",
			ids: []string{
				"arn:aws:organizations::329260820478:organization/o-meltunc7lj",
				"arn:aws:organizations::329260820478:organization/o-different1",
			},
			wantErr: false,
		},
		{
			name: "two account-level ARNs same org different accounts",
			ids: []string{
				"arn:aws:organizations::329260820478:account/o-melcpnc7lj/851725220677",
				"arn:aws:organizations::329260820478:account/o-melcpnc7lj/585132637328",
			},
			wantErr: false,
		},
		{
			name: "two OU ARNs with different OU IDs",
			ids: []string{
				"arn:aws:organizations::329260820478:ou/o-melcpnc7lj/ou-aaaa-11111111",
				"arn:aws:organizations::329260820478:ou/o-melcpnc7lj/ou-bbbb-22222222",
			},
			wantErr: false,
		},
		{
			name:    "empty list",
			ids:     []string{},
			wantErr: false,
		},
		{
			name:    "single entry",
			ids:     []string{"arn:aws:organizations::329260820478:organization/o-meltunc7lj"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateShareOrgIds(tt.ids)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateShareOrgIds(%v) error = %v, wantErr %v", tt.ids, err, tt.wantErr)
			}
		})
	}
}
