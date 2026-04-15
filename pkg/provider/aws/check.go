package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (a *aws) ImageExists(imageName string) (bool, string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return false, "", fmt.Errorf("error loading AWS config: %w", err)
	}
	client := ec2.NewFromConfig(cfg)

	tagFilterName := "tag:Name"
	output, err := client.DescribeImages(context.Background(), &ec2.DescribeImagesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   &tagFilterName,
				Values: []string{imageName},
			},
		},
		Owners: []string{"self"},
	})
	if err != nil {
		return false, "", fmt.Errorf("error describing images: %w", err)
	}
	if len(output.Images) > 0 {
		return true, *output.Images[0].ImageId, nil
	}
	return false, "", nil
}
