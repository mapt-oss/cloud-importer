package ibm

import (
	"context"
	"fmt"
	"os"

	"github.com/mapt-oss/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

const (
	configIBMAPIKey        = "ibmcloud:ibmcloud_api_key"
	configIBMRegion        = "ibmcloud:region"
	configIBMResourceGroup = "ibmcloud:resource_group"
)

var envCredentials = map[string]string{
	configIBMAPIKey:        "IBMCLOUD_API_KEY",
	configIBMRegion:        "IC_REGION",
	configIBMResourceGroup: "IC_RESOURCE_GROUP",
}

type ibmProvider struct{}

func Provider() *ibmProvider {
	return &ibmProvider{}
}

func (p *ibmProvider) GetProviderCredentials(customCredentials map[string]string) credentials.ProviderCredentials {
	return credentials.ProviderCredentials{
		SetCredentialFunc: SetIBMCredentials,
		FixedCredentials:  customCredentials,
	}
}

func SetIBMCredentials(ctx context.Context, stack auto.Stack, customCredentials map[string]string) error {
	return credentials.SetCredentials(ctx, stack, customCredentials, envCredentials)
}

func (p *ibmProvider) ValidateShareTargets(shareAccountIds []string) error {
	return validateShareAccountIds(shareAccountIds)
}

func validateShareAccountIds(ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate share target: account ID %q appears more than once in --share-orgs-ids", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (p *ibmProvider) DeleteLocks(backedURL string) {
	DeleteLocks(backedURL)
}

func (p *ibmProvider) CleanupState(backedURL string) {
	CleanupState(backedURL)
}

func sourceRegion() (string, error) {
	if r := os.Getenv("IC_REGION"); r != "" {
		return r, nil
	}
	if r := os.Getenv("IBMCLOUD_REGION"); r != "" {
		return r, nil
	}
	return "", fmt.Errorf("missing IBM Cloud region: set IC_REGION or IBMCLOUD_REGION")
}

func envCosInstanceID() (string, error) {
	if id := os.Getenv("IBMCLOUD_COS_INSTANCE_ID"); id != "" {
		return id, nil
	}
	if id := os.Getenv("IC_COS_INSTANCE_ID"); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("missing IBM COS instance ID: set IBMCLOUD_COS_INSTANCE_ID")
}

func envVPCOperatingSystem() (string, error) {
	if slug := os.Getenv("IBMCLOUD_VPC_OS"); slug != "" {
		return slug, nil
	}
	if slug := os.Getenv("IC_VPC_OS"); slug != "" {
		return slug, nil
	}
	return "", fmt.Errorf("missing IBM VPC OS slug: set IBMCLOUD_VPC_OS (e.g. red-hat-enterprise-linux-9-amd64-byol)")
}

func envResourceGroup() string {
	if rg := os.Getenv("IC_RESOURCE_GROUP"); rg != "" {
		return rg
	}
	return os.Getenv("IBMCLOUD_RESOURCE_GROUP")
}
