package aws

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/redhat-developer/mapt/pkg/provider/aws/data"
)

type replicateRequest struct {
	amiName       string
	targetRegions []string
}

func (a *aws) Replicate(amiName string, targetRegions []string) (pulumi.RunFunc, error) {
	r := replicateRequest{
		amiName,
		targetRegions}

	return r.runFunc, nil
}

func (r replicateRequest) runFunc(ctx *pulumi.Context) error {
	return replicateAMI(ctx, r.amiName)
}

func replicateAMI(ctx *pulumi.Context, amiName string) error {
	amiInfo, err := data.FindAMI(&amiName, nil)
	if err != nil {
		return err
	}

	if amiInfo == nil {
		return fmt.Errorf("Unable to find AMI %s", amiName)
	}

	_, err = ec2.NewAmiCopy(ctx,
		amiName,
		&ec2.AmiCopyArgs{
			Description: pulumi.String(
				fmt.Sprintf("Replica of %s from %s", *amiInfo.Image.ImageId, *amiInfo.Region)),
			SourceAmiId:     pulumi.String(*amiInfo.Image.ImageId),
			SourceAmiRegion: pulumi.String(*amiInfo.Region),
		},
		pulumi.RetainOnDelete(true))
	if err != nil {
		return err
	}

	return nil
}
