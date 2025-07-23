package aws

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type shareRequest struct {
	imageID         string
	targetAccountID string
}

func (a *aws) Share(imageID string, targetAccountID string) (pulumi.RunFunc, error) {
	r := shareRequest{
		imageID,
		targetAccountID}
	return r.runFunc, nil
}

func (r shareRequest) runFunc(ctx *pulumi.Context) error {
	return shareAMI(ctx, r.imageID, r.targetAccountID)
}

func shareAMI(ctx *pulumi.Context, imageID, targetAccountID string) error {
	snapshotId, err := getSnapshotID(ctx, imageID)
	if err != nil {
		return err
	}
	// Share the AMI with other AWS accounts
	_, err = ec2.NewAmiLaunchPermission(ctx,
		"amiLaunchPermission",
		&ec2.AmiLaunchPermissionArgs{
			ImageId:   pulumi.String(imageID),
			AccountId: pulumi.String(targetAccountID),
		})
	if err != nil {
		return err
	}
	// Share the snapshot(s) backing the AMI
	_, err = ec2.NewSnapshotCreateVolumePermission(ctx,
		"shareSnapshot",
		&ec2.SnapshotCreateVolumePermissionArgs{
			SnapshotId: pulumi.String(*snapshotId),
			AccountId:  pulumi.String(targetAccountID),
		})
	return err
}

func getSnapshotID(ctx *pulumi.Context, imageID string) (*string, error) {
	amiInfo, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
		Owners: []string{"self"},
		Filters: []ec2.GetAmiFilter{
			{
				Name:   "image-id",
				Values: []string{imageID},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(amiInfo.BlockDeviceMappings) != 1 {
		return nil, fmt.Errorf("can not find the device mapped to the AMI")
	}
	if snapshotID, ok := amiInfo.BlockDeviceMappings[0].Ebs["snapshot_id"]; ok {
		return &snapshotID, nil
	}
	return nil, fmt.Errorf("can not find the device mapped to the AMI")
}
