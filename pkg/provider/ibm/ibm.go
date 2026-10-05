package ibm

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/mapt-oss/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

const (
	configIBMAPIKey   = "ibmcloud:ibmcloud_api_key"
	configIBMRegion   = "ibmcloud:region"
)

var envCredentials = map[string]string{
	configIBMAPIKey:    "IBMCLOUD_API_KEY",
	configIBMRegion:    "IBMCLOUD_REGION",
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

// ibmAccountIDRe matches IBM Cloud account IDs: 32 lowercase hex characters.
var ibmAccountIDRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

func validateShareAccountIds(ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if !ibmAccountIDRe.MatchString(id) {
			return fmt.Errorf("invalid IBM Cloud account ID %q: expected 32 lowercase hex characters", id)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate share target: account ID %q appears more than once in --share-orgs-ids", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}


func (p *ibmProvider) PostRegisterPublish(registerOutputs auto.UpResult, catalogVersion string, shareOrgIds []string) error {
	if catalogVersion == "" {
		return nil
	}
	getStr := func(key string) (string, error) {
		out, ok := registerOutputs.Outputs[key]
		if !ok {
			return "", fmt.Errorf("PostRegisterPublish: output %q not found in register stack", key)
		}
		s, ok := out.Value.(string)
		if !ok {
			return "", fmt.Errorf("PostRegisterPublish: output %q is not a string", key)
		}
		return s, nil
	}
	catalogID, err := getStr("catalogID")
	if err != nil {
		return err
	}
	vpcImageID, err := getStr("vpcImageID")
	if err != nil {
		return err
	}
	vpcImageName, err := getStr("vpcImageName")
	if err != nil {
		return err
	}
	offeringName, err := getStr(outOfferingName)
	if err != nil {
		return err
	}
	region, err := sourceRegion()
	if err != nil {
		return err
	}
	return PublishVPCImage(catalogID, offeringName, vpcImageID, vpcImageName, catalogVersion, region, shareOrgIds)
}

func (p *ibmProvider) DeleteLocks(backedURL string) {
	DeleteLocks(backedURL)
}

func (p *ibmProvider) CleanupState(backedURL string) {
	CleanupState(backedURL)
}

func sourceRegion() (string, error) {
	if r := os.Getenv("IBMCLOUD_REGION"); r != "" {
		return r, nil
	}
	if r := os.Getenv("IC_REGION"); r != "" {
		return r, nil
	}
	return "", fmt.Errorf("missing IBM Cloud region: set IBMCLOUD_REGION (or legacy IC_REGION)")
}


// sncIBMSlug maps arch to the IBM VPC OS slug for SNC/OpenShift Local images (RHCOS-based).
var sncIBMSlug = map[string]string{
	"x86_64": "rhel-coreos-stable-amd64",
	"arm64":  "rhel-coreos-stable-arm64",
}

// rhelaiIBMSlug maps arch to the IBM VPC OS slug for RHELAI images.
// us-south only has rhel-coreos-stable-* slugs; IBM doesn't validate image content so this works as a workaround.
var rhelaiIBMSlug = map[string]string{
	"x86_64": "rhel-coreos-stable-amd64",
	"arm64":  "rhel-coreos-stable-arm64",
}

func sncVPCOperatingSystem(arch string) (string, error) {
	slug, ok := sncIBMSlug[arch]
	if !ok {
		return "", fmt.Errorf("unsupported arch %q for IBM VPC SNC OS slug: must be x86_64 or arm64", arch)
	}
	return slug, nil
}

func rhelaiVPCOperatingSystem(arch string) (string, error) {
	slug, ok := rhelaiIBMSlug[arch]
	if !ok {
		return "", fmt.Errorf("unsupported arch %q for IBM VPC RHELAI OS slug: must be x86_64 or arm64", arch)
	}
	return slug, nil
}

