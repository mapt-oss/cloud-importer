package aws

import (
	"github.com/redhat-developer/mapt/pkg/provider/aws/data"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type shareRequest struct {
	imageID         string
	targetAccountID string
	arch            string
	organizationARN string
}

func (a *aws) Share(imageID, arch, targetAccountID, organizationARN string) (pulumi.RunFunc, []string, error) {
	r := shareRequest{
		imageID,
		targetAccountID,
		arch,
		organizationARN}

	regions, err := data.GetRegions()
	if err != nil {
		return nil, nil, err
	}
	return r.runFunc, regions, err
}

func (r shareRequest) runFunc(ctx *pulumi.Context) error {
	return shareAMI(ctx, r.imageID, r.arch, r.targetAccountID, r.organizationARN)
}

func shareAMI(ctx *pulumi.Context, imageID, arch, targetAccountID, organizationARN string) error {
	img, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
		MostRecent: pulumi.BoolRef(true),
		NameRegex:  pulumi.StringRef(`^openshift-local-[0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,2}-.*`),
		Owners: []string{
			"self",
		},
		Filters: []ec2.GetAmiFilter{
			{
				Name: "name",
				Values: []string{
					imageID,
				},
			},
			{
				Name: "architecture",
				Values: []string{
					arch,
				},
			},
		},
	}, nil)
	if err != nil {
		return err
	}
	launchPermissionArgs := &ec2.AmiLaunchPermissionArgs{
		ImageId: pulumi.String(img.ImageId),
	}

	if len(organizationARN) > 0 {
		launchPermissionArgs.OrganizationArn = pulumi.String(organizationARN)
	} else {
		launchPermissionArgs.AccountId = pulumi.String(targetAccountID)
	}

	_, err = ec2.NewAmiLaunchPermission(ctx, "amiLaunchPermission", launchPermissionArgs)
	if err != nil {
		return err
	}
	return nil
}
