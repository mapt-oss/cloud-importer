package azure

import (
	"context"
	"strings"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/credentials"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	CONFIG_AZURE_NATIVE_LOCATION        string = "azure-native:location"
	CONFIG_AZURE_NATIVE_SUBSCRIPTION_ID string = "azure-native:subscriptionId"
)

// pulumi config key : azure-native env credential
var envCredentials = map[string]string{
	CONFIG_AZURE_NATIVE_LOCATION:        "ARM_LOCATION_NAME",
	CONFIG_AZURE_NATIVE_SUBSCRIPTION_ID: "ARM_SUBSCRIPTION_ID",
}

type azureProvider struct{}

func Provider() *azureProvider {
	return &azureProvider{}
}

func (p *azureProvider) RHELAI(rawImageFilePath, amiName string) (pulumi.RunFunc, error) {
	rhelAIReq := rhelAIRequest{
		vhdPath:   rawImageFilePath,
		imageName: amiName,
	}
	return rhelAIReq.runFunc, nil
}

func (p *azureProvider) Share(imageID string, targetAccountID string) (pulumi.RunFunc, error) {
	// Not implemented for Azure
	return nil, nil
}

func (p *azureProvider) OpenshiftLocal(bundleURL, shasumURL, arch string, regions []string) (pulumi.RunFunc, error) {
	ocpReq := openshiftRequest{
		bundleURL: bundleURL,
		shasumURL: shasumURL,
		arch:      arch,
		regions:   regions,
	}
	return ocpReq.runFunc, nil
}

func (p *azureProvider) Replicate(amiName string, targetRegions []string) (pulumi.RunFunc, []string, error) {
	var availableRegions []string
	var err error

	if len(targetRegions) > 0 {
		if strings.Contains(targetRegions[0], "all") {
			availableRegions, err = Locations()
			if err != nil {
				logging.Debugf("Unable to get list of all locations: %v", err)
				return nil, []string{}, err
			}
		} else {
			availableRegions = targetRegions
		}
	}

	req := replicateRequest{
		galleryImageName: amiName,
		targetRegions:    availableRegions,
	}
	return req.runFunc, availableRegions, nil
}

func (p *azureProvider) GetProviderCredentials(customCredentials map[string]string) credentials.ProviderCredentials {
	return credentials.ProviderCredentials{
		SetCredentialFunc: SetAzureCredentials,
		FixedCredentials:  customCredentials}
}

func SetAzureCredentials(ctx context.Context, stack auto.Stack, customCredentials map[string]string) error {
	return credentials.SetCredentials(ctx, stack, customCredentials, envCredentials)
}
