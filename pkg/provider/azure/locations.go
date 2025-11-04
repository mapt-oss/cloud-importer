package azure

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
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
	client, err := armresources.NewProvidersClient(*subscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	provider, err := client.Get(ctx, "Microsoft.Compute", nil)
	if err != nil {
		return nil, err
	}
	var supportedLocations []string
	for _, resourceType := range provider.ResourceTypes {
		if *resourceType.ResourceType == "galleries" ||
			*resourceType.ResourceType == "galleries/images" ||
			*resourceType.ResourceType == "galleries/images/versions" {
			for _, location := range resourceType.Locations {
				normalizedLoc := strings.ToLower(strings.ReplaceAll(*location, " ", ""))
				supportedLocations = append(supportedLocations, normalizedLoc)
			}
			break
		}
	}
	seen := make(map[string]bool)
	var uniqueLocations []string
	for _, loc := range supportedLocations {
		if !seen[loc] {
			seen[loc] = true
			uniqueLocations = append(uniqueLocations, loc)
		}
	}

	return uniqueLocations, nil
}
