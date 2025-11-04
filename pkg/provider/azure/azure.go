package azure

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/credentials"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

const (
	CONFIG_AZURE_NATIVE_LOCATION        string = "azure-native:location"
	CONFIG_AZURE_NATIVE_SUBSCRIPTION_ID string = "azure-native:subscriptionId"
)

var azIdentityEnvs = []string{
	"AZURE_TENANT_ID",
	"AZURE_SUBSCRIPTION_ID",
	"AZURE_CLIENT_ID",
	"AZURE_CLIENT_SECRET",
}

// pulumi config key : azure-native env credential
var envCredentials = map[string]string{
	CONFIG_AZURE_NATIVE_LOCATION:        "ARM_LOCATION_NAME",
	CONFIG_AZURE_NATIVE_SUBSCRIPTION_ID: "ARM_SUBSCRIPTION_ID",
}

type azureProvider struct{}

func Provider() *azureProvider {
	setAZIdentityEnvs()
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

// Envs required for auth with go sdk
// https://learn.microsoft.com/es-es/azure/developer/go/azure-sdk-authentication?tabs=bash#service-principal-with-a-secret
// do not match standard envs for pulumi envs for auth with native sdk
// https://www.pulumi.com/registry/packages/azure-native/installation-configuration/#set-configuration-using-environment-variables
func setAZIdentityEnvs() {
	for _, e := range azIdentityEnvs {
		if err := os.Setenv(e,
			os.Getenv(strings.ReplaceAll(e, "AZURE", "ARM"))); err != nil {
			logging.Error(err)
		}
	}
}

func sourceHostingPlace() (*string, error) {
	hp := os.Getenv("ARM_LOCATION_NAME")
	if len(hp) > 0 {
		return &hp, nil
	}
	return nil, fmt.Errorf("missing default value for AWS Region")
}
