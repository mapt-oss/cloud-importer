package azure

import (
	"context"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

const ENV_AZURE_SUBSCRIPTION_ID = "ARM_SUBSCRIPTION_ID"

func Locations() ([]string, error) {
	cred, subscriptionID, err := getCredentials()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	client, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		return nil, err
	}
	var locations []string
	var noExtendedLocations = false

	pager := client.NewListLocationsPager(*subscriptionID, &armsubscriptions.ClientListLocationsOptions{
		IncludeExtendedLocations: &noExtendedLocations,
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, loc := range page.Value {
			locations = append(locations, *loc.Name)
		}
	}
	return locations, nil
}

func getCredentials() (cred *azidentity.DefaultAzureCredential, subscriptionID *string, err error) {
	cred, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return
	}
	azSubsID := os.Getenv(ENV_AZURE_SUBSCRIPTION_ID)
	subscriptionID = &azSubsID
	return
}
