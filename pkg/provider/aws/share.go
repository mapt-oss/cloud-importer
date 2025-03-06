package aws

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type shareRequest struct {
	imageID         string
	targetAccountID string
}

func (a *Provider) ShareImage(imageID string, targetAccountID string) (pulumi.RunFunc, error) {
	r := shareRequest{
		imageID,
		targetAccountID}
	return r.runFunc, nil
}

func (r shareRequest) runFunc(ctx *pulumi.Context) error {
	return shareAMI(ctx, r.imageID, r.targetAccountID)
}

func shareAMI(ctx *pulumi.Context, imageID, targetAccountID string) error {
	// Share the AMI with other AWS accounts
	_, err := ec2.NewAmiLaunchPermission(ctx,
		"amiLaunchPermission",
		&ec2.AmiLaunchPermissionArgs{
			ImageId:   pulumi.String(imageID),
			AccountId: pulumi.String(targetAccountID),
		})
	if err != nil {
		return err
	}
	return nil
}
