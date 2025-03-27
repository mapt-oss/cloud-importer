package aws

import (
	"crypto/rand"
	"fmt"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	"github.com/pulumi/pulumi-aws-native/sdk/go/aws/iam"
	"github.com/pulumi/pulumi-aws-native/sdk/go/aws/s3"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ebs"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// This function creates a temporary bucket to upload the disk image to be imported
// It returns the bucket resource, the generated bucket name and error if any
func bucketEphemeral(ctx *pulumi.Context, bucketName *string) (*s3.Bucket, error) {
	return s3.NewBucket(ctx,
		"s3EphemeralBucket",
		&s3.BucketArgs{
			BucketName: pulumi.String(*bucketName),
			// https://aws.amazon.com/blogs/aws/heads-up-amazon-s3-security-changes-are-coming-in-april-of-2023/
			OwnershipControls: s3.BucketOwnershipControlsArgs{
				Rules: s3.BucketOwnershipControlsRuleArray{
					s3.BucketOwnershipControlsRuleArgs{
						ObjectOwnership: s3.BucketOwnershipControlsRuleObjectOwnershipObjectWriter,
					},
				},
			},
			Tags: context.GetTags(),
		})

}

// // random name for temporary assets required for importing the image
func randomID() *string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	id := fmt.Sprintf("cloud-importer-%x", b)
	return &id
}

// https://docs.aws.amazon.com/vm-import/latest/userguide/required-permissions.html
func createVMIEmportExportRole(ctx *pulumi.Context,
	roleName *string) (*iam.Role, pulumi.Resource, error) {
	role, err := iam.NewRole(ctx,
		"role",
		&iam.RoleArgs{
			RoleName:                 pulumi.String(*roleName),
			AssumeRolePolicyDocument: pulumi.Any(trustPolicyContent()),
		})
	if err != nil {
		return nil, nil, err
	}
	rolePolicyAttachment, err := iam.NewRolePolicy(ctx,
		"rolePolicy",
		&iam.RolePolicyArgs{
			RoleName: role.ID(),
			PolicyDocument: pulumi.Any(
				rolePolicyContent(*roleName)),
		})
	return role, rolePolicyAttachment, err
}

func uploadDisk(ctx *pulumi.Context, rawImageFilePath, bucketName *string,
	dependecies []pulumi.Resource) (pulumi.Resource, error) {
	// aws s3 cp %s s3://%s/disk.raw
	uploadCommand :=
		fmt.Sprintf(
			"aws s3 cp %s s3://%s/disk.raw --only-show-error",
			*rawImageFilePath,
			*bucketName)
	deleteCommand :=
		fmt.Sprintf(
			"aws s3 rm s3://%s/disk.raw --only-show-error",
			*bucketName)

	return local.NewCommand(ctx,
		"upload",
		&local.CommandArgs{
			Create: pulumi.String(uploadCommand),
			Delete: pulumi.String(deleteCommand),
		},
		pulumi.Timeouts(
			&pulumi.CustomTimeouts{
				Create: "40m",
				Update: "40m",
				Delete: "40m"}),
		pulumi.DependsOn(dependecies))
}

// from an image as a raw on a s3 bucket this function will import it as a snapshot
// and the register the snapshot as an AMI
func registerAMI(ctx *pulumi.Context, amiName *string, arch *string,
	bucketName *string, vmieRole *iam.Role,
	dependsOn []pulumi.Resource) (*ec2.Ami, error) {
	snapshot, err := ebs.NewSnapshotImport(ctx,
		"snapshot",
		&ebs.SnapshotImportArgs{
			Description: pulumi.String(*amiName),
			DiskContainer: &ebs.SnapshotImportDiskContainerArgs{
				Format: pulumi.String("RAW"),
				UserBucket: &ebs.SnapshotImportDiskContainerUserBucketArgs{
					S3Bucket: pulumi.String(*bucketName),
					S3Key:    pulumi.String("disk.raw"),
				},
			},
			RoleName: vmieRole.RoleName,
		},
		pulumi.DependsOn(dependsOn),
		// This allows to mask the import operation with a create and destroy
		// keeping only the AMI the other resources are ephermeral only tied to
		// the execution
		pulumi.RetainOnDelete(true))
	if err != nil {
		return nil, err
	}
	return ec2.NewAmi(ctx,
		"ami",
		&ec2.AmiArgs{
			EbsBlockDevices: ec2.AmiEbsBlockDeviceArray{
				&ec2.AmiEbsBlockDeviceArgs{
					DeviceName: pulumi.String("/dev/xvda"),
					SnapshotId: snapshot.ID(),
					VolumeSize: pulumi.Int(1000),
				},
			},
			Name:               pulumi.String(*amiName),
			Description:        pulumi.String(*amiName),
			RootDeviceName:     pulumi.String("/dev/xvda"),
			VirtualizationType: pulumi.String("hvm"),
			Architecture:       pulumi.String(*arch),
			// Required by c6a instances
			EnaSupport: pulumi.Bool(true),
		},
		// This allows to mask the import operation with a create and destroy
		// keeping only the AMI the other resources are ephermeral only tied to
		// the execution
		pulumi.RetainOnDelete(true))
}

func trustPolicyContent() map[string]interface{} {
	return map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Sid":    "",
				"Effect": "Allow",
				"Principal": map[string]interface{}{
					"Service": "vmie.amazonaws.com",
				},
				"Action": "sts:AssumeRole",
				"Condition": map[string]interface{}{
					"StringEquals": map[string]interface{}{
						"sts:ExternalId": "vmimport",
					},
				},
			},
		},
	}
}

// TODO review s3 actions
func rolePolicyContent(bucketName string) map[string]interface{} {
	bucketNameARN := fmt.Sprintf("arn:aws:s3:::%s", bucketName)
	bucketNameRecursiveARN := fmt.Sprintf("arn:aws:s3:::%s/*", bucketName)
	return map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect": "Allow",
				"Action": []string{
					"s3:GetBucketLocation",
					"s3:GetObject",
					"s3:ListBucket",
				},
				"Resource": []string{
					bucketNameARN,
					bucketNameRecursiveARN,
				},
			},
			{
				"Effect": "Allow",
				"Action": []string{
					"ec2:ModifySnapshotAttribute",
					"ec2:CopySnapshot",
					"ec2:RegisterImage",
					"ec2:Describe*",
				},
				"Resource": "*",
			},
		},
	}
}
