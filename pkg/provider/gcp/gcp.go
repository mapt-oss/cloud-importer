package gcp

import (
	"context"
	"fmt"
	"os"

	"github.com/mapt-oss/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

const (
	configGCPProject     = "gcp:project"
	configGCPCredentials = "gcp:credentials"
	configGCPRegion      = "gcp:region"
)

var envCredentials = map[string]string{
	configGCPProject:     "GOOGLE_PROJECT",
	configGCPCredentials: "GOOGLE_CREDENTIALS",
	configGCPRegion:      "GOOGLE_REGION",
}

type gcpProvider struct{}

func Provider() *gcpProvider {
	return &gcpProvider{}
}

func (p *gcpProvider) ValidateShareTargets(_ []string) error { return nil }

func (p *gcpProvider) DeleteLocks(backedURL string) { DeleteLocks(backedURL) }

func (p *gcpProvider) CleanupState(backedURL string) { CleanupState(backedURL) }

func (p *gcpProvider) GetProviderCredentials(customCredentials map[string]string) credentials.ProviderCredentials {
	return credentials.ProviderCredentials{
		SetCredentialFunc: SetGCPCredentials,
		FixedCredentials:  customCredentials,
	}
}

func SetGCPCredentials(ctx context.Context, stack auto.Stack, customCredentials map[string]string) error {
	return credentials.SetCredentials(ctx, stack, customCredentials, envCredentials)
}

func sourceHostingPlace() (*string, error) {
	hp := os.Getenv("GOOGLE_REGION")
	if len(hp) > 0 {
		return &hp, nil
	}
	return nil, fmt.Errorf("missing GCP region, set GOOGLE_REGION")
}
