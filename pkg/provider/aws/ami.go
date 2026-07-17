package aws

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mapt-oss/cloud-importer/pkg/manager/context"
	hostingPlaces "github.com/mapt-oss/cloud-importer/pkg/util/hosting-place"
	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
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
	outBucketName = "bucketname"
)

func (a *aws) ValidateShareTargets(shareOrgIds []string) error {
	return validateShareOrgIds(shareOrgIds)
}

func (a *aws) ImageRegister(ephemeralResults auto.UpResult, replicate bool, shareOrgIds []string) (pulumi.RunFunc, func(gocontext.Context), error) {
	if err := validateShareOrgIds(shareOrgIds); err != nil {
		return nil, nil, err
	}
	amiNameOutput, ok := ephemeralResults.Outputs[outAMIName]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outAMIName)
	}
	amiArchOutput, ok := ephemeralResults.Outputs[outAMIArch]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outAMIArch)
	}
	bucketNameOutput, ok := ephemeralResults.Outputs[outBucketName]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outBucketName)
	}
	roleNameOputput, ok := ephemeralResults.Outputs[outRoleName]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outRoleName)
	}
	r := registerRequest{
		name:         amiNameOutput.Value.(string),
		arch:         amiArchOutput.Value.(string),
		bucketName:   bucketNameOutput.Value.(string),
		vmIERoleName: roleNameOputput.Value.(string),
		replicate:    replicate,
		shareorgARNs: shareOrgIds,
	}
	return r.registerFunc, r.monitorSnapshotImportProgress, nil
}

type registerRequest struct {
	name         string
	arch         string
	bucketName   string
	vmIERoleName string
	replicate    bool
	shareorgARNs []string
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
				shareOrgARNs: r.shareorgARNs,
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
			Tags:     pulumi.ToStringMap(context.GetTagsMap()),
		})
	if err != nil {
		return nil, err
	}
	ami, err := ec2.NewAmi(ctx,
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
			Tags:       pulumi.ToStringMap(mergeTags(map[string]string{"Name": r.name})),
		})
	if err != nil {
		return nil, err
	}
	for _, orgArn := range r.shareorgARNs {
		lArgs := launchPermArgs(ami.ID(), orgArn)
		_, err = ec2.NewAmiLaunchPermission(
			ctx,
			fmt.Sprintf("%s-%s", r.name, pulumiResourceId(orgArn)),
			lArgs)
		if err != nil {
			return nil, err
		}
	}
	return ami, nil
}

func (r *registerRequest) monitorSnapshotImportProgress(ctx gocontext.Context) {
	time.Sleep(30 * time.Second)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logging.Debugf("snapshot import progress monitor: failed to load AWS config: %v", err)
		return
	}
	client := awsec2.NewFromConfig(cfg)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	lastLog := ""
	snapshotDone := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !snapshotDone {
				snapshotDone = r.pollSnapshotImport(ctx, client, &lastLog)
			}
			if snapshotDone && r.replicate {
				r.pollAMIReplication(ctx, &lastLog)
			}
		}
	}
}

func (r *registerRequest) pollSnapshotImport(ctx gocontext.Context, client *awsec2.Client, lastLog *string) bool {
	resp, err := client.DescribeImportSnapshotTasks(ctx, &awsec2.DescribeImportSnapshotTasksInput{})
	if err != nil {
		logging.Debugf("snapshot import progress poll: %v", err)
		return false
	}
	for _, task := range resp.ImportSnapshotTasks {
		if task.Description == nil || *task.Description != r.name {
			continue
		}
		if task.SnapshotTaskDetail == nil {
			continue
		}
		detail := task.SnapshotTaskDetail
		status := ""
		if detail.Status != nil {
			status = *detail.Status
		}
		statusMsg := ""
		if detail.StatusMessage != nil {
			statusMsg = *detail.StatusMessage
		}
		progress := ""
		if detail.Progress != nil {
			progress = *detail.Progress
		}
		if status == "completed" {
			progress = "100"
		}
		var msg string
		if progress != "" {
			msg = fmt.Sprintf("Snapshot import progress: status=%s, %s%% completed", status, progress)
		} else if statusMsg != "" {
			msg = fmt.Sprintf("Snapshot import progress: status=%s, %s", status, statusMsg)
		} else {
			msg = fmt.Sprintf("Snapshot import progress: status=%s", status)
		}
		if msg != *lastLog {
			*lastLog = msg
			logging.Info(msg)
		}
		return status == "completed"
	}
	return false
}

func (r *registerRequest) pollAMIReplication(ctx gocontext.Context, lastLog *string) {
	regions, err := getOtherRegions()
	if err != nil {
		logging.Debugf("AMI replication progress: failed to get regions: %v", err)
		return
	}
	tagFilterName := "tag:Name"
	available := 0
	pending := []string{}
	for _, region := range regions {
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			logging.Debugf("AMI replication progress poll (%s): %v", region, err)
			continue
		}
		client := awsec2.NewFromConfig(cfg)
		resp, err := client.DescribeImages(ctx, &awsec2.DescribeImagesInput{
			Filters: []ec2Types.Filter{
				{
					Name:   &tagFilterName,
					Values: []string{r.name},
				},
			},
			Owners: []string{"self"},
		})
		if err != nil {
			logging.Debugf("AMI replication progress poll (%s): %v", region, err)
			pending = append(pending, region)
			continue
		}
		if len(resp.Images) == 0 || resp.Images[0].State != ec2Types.ImageStateAvailable {
			pending = append(pending, region)
		} else {
			available++
		}
	}
	total := len(regions)
	var msg string
	if len(pending) == 0 {
		msg = fmt.Sprintf("AMI replication progress: %d/%d regions completed", available, total)
	} else {
		msg = fmt.Sprintf("AMI replication progress: %d/%d regions completed, pending: %s",
			available, total, strings.Join(pending, ", "))
	}
	if msg != *lastLog {
		*lastLog = msg
		logging.Info(msg)
	}
}

type replicateArgs struct {
	ctx          *pulumi.Context
	name         *string
	ami          *ec2.Ami
	sourceRegion *string
	shareOrgARNs []string
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
			Tags:            pulumi.ToStringMap(mergeTags(map[string]string{"Name": *args.name})),
		})
	if err != nil {
		hostingPlaces.SendAsyncErr(c, err)
		return
	}
	var amiPermissions *ec2.AmiLaunchPermission
	for _, orgArn := range args.shareOrgARNs {
		lArgs := launchPermArgs(ami.ID(), orgArn)
		lArgs.Region = pulumi.String(targetRegion)
		amiPermissions, err = ec2.NewAmiLaunchPermission(
			args.ctx,
			fmt.Sprintf("%s-%s-%s", *args.name, targetRegion, pulumiResourceId(orgArn)),
			lArgs)
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

// validateShareOrgIds returns an error if any two entries in ids resolve to the
// same orgId key, which would cause a duplicate Pulumi URN at resource creation
// time. Note: a plain account ID (e.g. "851725220677") and a full account-level
// ARN for the same account produce different keys and are not detected as
// duplicates here; normalising across those formats would require an
// organisations:DescribeAccount lookup that may not be available to the caller.
func validateShareOrgIds(ids []string) error {
	seen := make(map[string]string, len(ids))
	for _, id := range ids {
		key := pulumiResourceId(id)
		if prev, exists := seen[key]; exists {
			return fmt.Errorf(
				"duplicate share target: %q and %q resolve to the same identifier %q — "+
					"check for duplicate entries or conflicting management account IDs in org ARNs",
				prev, id, key)
		}
		seen[key] = id
	}
	return nil
}

// pulumiResourceId returns a string used only as a Pulumi resource name suffix —
// it is never sent to AWS. The full original ARN is passed to AWS unchanged via
// launchPermArgs.
//
// The suffix is derived from the ARN's resource path segments, intentionally
// excluding the management account ID (MGMT). MGMT is excluded because it does
// not change the AWS share target: an organization has exactly one management
// account, so two org ARNs with the same org ID but different MGMT values
// represent the same AWS target (one MGMT value must be wrong). Excluding MGMT
// means validateShareOrgIds correctly identifies such pairs as duplicates and
// returns a clear error rather than silently creating two Pulumi resources and
// letting AWS reject the one with the bad MGMT.
//
// Accepted input forms and their output:
//
//	Plain account ID   "851725220677"                                        → "851725220677"
//	Org-level ARN      "arn:aws:organizations::MGMT:organization/o-xxx"      → "o-xxx"
//	Account-level ARN  "arn:aws:organizations::MGMT:account/o-xxx/ACCT_ID"  → "o-xxx-ACCT_ID"
//	OU-level ARN       "arn:aws:organizations::MGMT:ou/o-xxx/ou-yyy-zzz"    → "o-xxx-ou-yyy-zzz"
//
// Malformed ARNs (starting with "arn:" but with too few colon-separated segments
// or no "/" in the resource part) are returned as-is to avoid a panic; they will
// surface an error at the Pulumi or AWS layer.
func pulumiResourceId(arn string) string {
	if !strings.HasPrefix(arn, "arn:") {
		return arn
	}
	segments := strings.Split(arn, ":")
	if len(segments) < 6 {
		return arn
	}
	parts := strings.Split(segments[5], "/")
	if len(parts) < 2 {
		return arn
	}
	return strings.Join(parts[1:], "-")
}

// launchPermArgs builds AmiLaunchPermissionArgs routing to the correct field.
// Accepts a plain 12-digit account ID or an organizations ARN
// (organization→OrganizationArn, ou→OrganizationalUnitArn, account→AccountId).
func launchPermArgs(imageId pulumi.StringInput, arn string) *ec2.AmiLaunchPermissionArgs {
	args := &ec2.AmiLaunchPermissionArgs{ImageId: imageId}
	if !strings.HasPrefix(arn, "arn:") {
		args.AccountId = pulumi.String(arn)
		return args
	}
	parts := strings.Split(strings.Split(arn, ":")[5], "/")
	switch parts[0] {
	case "account":
		args.AccountId = pulumi.String(parts[len(parts)-1])
	case "ou":
		args.OrganizationalUnitArn = pulumi.String(arn)
	default: // "organization"
		args.OrganizationArn = pulumi.String(arn)
	}
	return args
}

// mergeTags combines context tags with resource-specific tags.
// Resource-specific tags take precedence over context tags.
func mergeTags(resourceTags map[string]string) map[string]string {
	merged := make(map[string]string)

	// First, add all context tags
	for k, v := range context.GetTagsMap() {
		merged[k] = v
	}

	// Then, overlay resource-specific tags (these override context tags)
	for k, v := range resourceTags {
		merged[k] = v
	}

	return merged
}
