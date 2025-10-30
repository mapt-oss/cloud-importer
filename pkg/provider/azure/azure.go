package azure

import (
	"context"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
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

func (p *azureProvider) GetProviderCredentials(customCredentials map[string]string) credentials.ProviderCredentials {
	return credentials.ProviderCredentials{
		SetCredentialFunc: SetAzureCredentials,
		FixedCredentials:  customCredentials}
}

func SetAzureCredentials(ctx context.Context, stack auto.Stack, customCredentials map[string]string) error {
	return credentials.SetCredentials(ctx, stack, customCredentials, envCredentials)
}
