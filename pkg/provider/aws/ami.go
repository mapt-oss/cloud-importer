package aws

import (
	"fmt"

	hostingPlaces "github.com/devtools-qe-incubator/cloud-importer/pkg/util/hosting-place"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ebs"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var (
	// ImageRegister should get this values from the ephemeralResults
	outAMIName    = "aminame"
	outAMIArch    = "amiarch"
	outRoleName   = "rolename"
	outBucketName = "rolename"
)

func (a *aws) ImageRegister(ephemeralResults auto.UpResult, replicate bool, orgId string) (pulumi.RunFunc, error) {
	amiNameOutput, ok := ephemeralResults.Outputs[outAMIName]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outAMIName)
	}
	amiArchOutput, ok := ephemeralResults.Outputs[outAMIArch]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outAMIArch)
	}
	bucketNameOutput, ok := ephemeralResults.Outputs[outBucketName]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outBucketName)
	}
	roleNameOputput, ok := ephemeralResults.Outputs[outRoleName]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outRoleName)
	}
	r := registerRequest{
		name:         amiNameOutput.Value.(string),
		arch:         amiArchOutput.Value.(string),
		bucketName:   bucketNameOutput.Value.(string),
		vmIERoleName: roleNameOputput.Value.(string),
		replicate:    replicate,
		orgARN:       &orgId,
	}
	return r.registerFunc, nil
}

type registerRequest struct {
	name         string
	arch         string
	bucketName   string
	vmIERoleName string
	replicate    bool
	orgARN       *string
}

// from an image as a raw on a s3 bucket this function will import it as a snapshot
// and the register the snapshot as an AMI
func (r *registerRequest) registerFunc(ctx *pulumi.Context) error {
	ami, err := r.newAMI(ctx)
	if err != nil {
		return err
	}
	if r.replicate {
		regions, err := getOtherRegions()
		if err != nil {
			return err
		}
		sourceRegion, err := sourceHostingPlace()
		if err != nil {
			return err
		}
		_, err = hostingPlaces.RunOnHostingPlaces(regions,
			replicateArgs{
				ctx:          ctx,
				name:         &r.name,
				ami:          ami,
				sourceRegion: sourceRegion,
				orgARN:       r.orgARN,
			},
			replicaAsync)
		return err
	}
	return err
}

func (r *registerRequest) newAMI(ctx *pulumi.Context) (*ec2.Ami, error) {
	snapshot, err := ebs.NewSnapshotImport(ctx,
		"snapshot",
		&ebs.SnapshotImportArgs{
			Description: pulumi.String(r.name),
			DiskContainer: &ebs.SnapshotImportDiskContainerArgs{
				Format: pulumi.String("RAW"),
				UserBucket: &ebs.SnapshotImportDiskContainerUserBucketArgs{
					S3Bucket: pulumi.String(r.bucketName),
					S3Key:    pulumi.String("disk.raw"),
				},
			},
			RoleName: pulumi.String(r.vmIERoleName),
		})
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
					VolumeType: pulumi.String("gp3"),
					Iops:       pulumi.Int(3000),
				},
			},
			Name:               pulumi.String(r.name),
			Description:        pulumi.String(r.name),
			RootDeviceName:     pulumi.String("/dev/xvda"),
			VirtualizationType: pulumi.String("hvm"),
			Architecture:       pulumi.String(r.arch),
			// Required by c6a instances
			EnaSupport: pulumi.Bool(true),
			Tags: pulumi.ToStringMap(map[string]string{
				"Name": r.name}),
		})
}

type replicateArgs struct {
	ctx          *pulumi.Context
	name         *string
	ami          *ec2.Ami
	sourceRegion *string
	orgARN       *string
}

type replicateResults struct {
	ami            *ec2.AmiCopy
	amiPermissions *ec2.AmiLaunchPermission
}

func replicaAsync(targetRegion string, args replicateArgs, c chan hostingPlaces.HostingPlaceData[replicateResults]) {
	ami, err := ec2.NewAmiCopy(args.ctx,
		fmt.Sprintf("%s-%s", *args.name, targetRegion),
		&ec2.AmiCopyArgs{
			Description:     pulumi.String(fmt.Sprintf("%s replica ", *args.name)),
			SourceAmiId:     args.ami.ID(),
			SourceAmiRegion: pulumi.String(*args.sourceRegion),
			Region:          pulumi.String(targetRegion),
			Tags: pulumi.ToStringMap(map[string]string{
				"Name": *args.name}),
		})
	if err != nil {
		hostingPlaces.SendAsyncErr(c, err)
		return
	}
	var amiPermissions *ec2.AmiLaunchPermission
	if args.orgARN != nil {
		amiPermissions, err = ec2.NewAmiLaunchPermission(
			args.ctx,
			fmt.Sprintf("%s-%s", *args.name, targetRegion),
			&ec2.AmiLaunchPermissionArgs{
				ImageId:         ami.ID(),
				OrganizationArn: pulumi.String(*args.orgARN),
				Region:          pulumi.String(targetRegion),
			})
		if err != nil {
			hostingPlaces.SendAsyncErr(c, err)
			return
		}
	}
	c <- hostingPlaces.HostingPlaceData[replicateResults]{
		Region: targetRegion,
		Value: replicateResults{
			ami:            ami,
			amiPermissions: amiPermissions,
		},
		Err: nil}
}
