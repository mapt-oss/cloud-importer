package aws

import (
	"context"
	"slices"

	"github.com/redhat-developer/mapt/pkg/util"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var (
	optInStatusFilter      string = "opt-in-status"
	OptInStatusNotRequired string = "opt-in-not-required"
	OptInStatusOptedIn     string = "opted-in"
)

func getOtherRegions() ([]string, error) {
	allRegions, err := getRegions()
	if err != nil {
		return nil, err
	}
	currentRegion, err := sourceHostingPlace()
	if err != nil {
		return nil, err
	}
	idx := slices.Index(allRegions, *currentRegion)
	return append(allRegions[:idx], allRegions[idx+1:]...), nil
}

func getRegions() ([]string, error) {
	return getRegionsByOptInStatus([]string{OptInStatusNotRequired, OptInStatusOptedIn})
}

func getRegionsByOptInStatus(optInStaus []string) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)
	regions, err := client.DescribeRegions(
		context.Background(),
		&ec2.DescribeRegionsInput{
			Filters: []ec2Types.Filter{
				{
					Name:   &optInStatusFilter,
					Values: optInStaus,
				},
			}})
	if err != nil {
		return nil, err
	}
	return util.ArrayConvert(regions.Regions,
			func(item ec2Types.Region) string {
				return *item.RegionName
			}),
		nil
}
