package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/pulumi/pulumi-azure-native-sdk/compute/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func targetRegions() (compute.TargetRegionArray, error) {
	locations, err := locations()
	if err != nil {
		return nil, err
	}
	targetRegionsArgs := sliceConvert(locations, func(location string) *compute.TargetRegionArgs {
		return &compute.TargetRegionArgs{
			Name:                 pulumi.String(location),
			RegionalReplicaCount: pulumi.Int(1),
			ExcludeFromLatest:    pulumi.Bool(false),
		}
	})
	var targetRegions = compute.TargetRegionArray{}
	for _, r := range targetRegionsArgs {
		targetRegions = append(targetRegions, r)
	}
	return targetRegions, nil
}

func locations() ([]string, error) {
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
	pager := client.NewListLocationsPager(*subscriptionID, nil)
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
