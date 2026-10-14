package ibm

import (
	gocontext "context"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/manager/context"
	ibmcloud "github.com/mapt-oss/pulumi-ibmcloud/sdk/go/ibmcloud"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ibmcloudProviderVersion returns the version of the pulumi-ibmcloud SDK as
// recorded in the build's module graph, so it stays in sync with go.mod
// without any hardcoding.
func ibmcloudProviderVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/mapt-oss/pulumi-ibmcloud/sdk" {
			return strings.TrimPrefix(dep.Version, "v")
		}
	}
	return ""
}

// replicaGroup represents a set of VPC regions that all import from the same
// COS bucket. Cross-region geos allow multiple VPC regions to share one bucket
// (no per-region copy); regions without a geo each get their own bucket.
type replicaGroup struct {
	cosURI             string   // cos://geo/bucket/disk.qcow2 or cos://region/bucket/disk.qcow2
	bucket             string   // COS bucket name
	endpoint           string   // COS S3 endpoint for bucket operations
	locationConstraint string   // LocationConstraint for bucket creation; empty when reusing primary
	regions            []string // VPC regions that import from this bucket
	needsCopy          bool     // true: stream from primary bucket; false: reuse primary bucket
}

func (p *ibmProvider) ImageRegister(ephemeralResults auto.UpResult, replicate bool, _ []string) (pulumi.RunFunc, func(gocontext.Context), error) {
	imageNameOutput, ok := ephemeralResults.Outputs[outImageName]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outImageName)
	}
	cosURIOutput, ok := ephemeralResults.Outputs[outCOSURI]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outCOSURI)
	}
	osSlugOutput, ok := ephemeralResults.Outputs[outOSSlug]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outOSSlug)
	}
	bucketNameOutput, ok := ephemeralResults.Outputs[outBucketName]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outBucketName)
	}

	existingPolicyID, err := findVPCCOSAuthPolicy()
	if err != nil {
		existingPolicyID = ""
	}

	primaryRegion, err := sourceRegion()
	if err != nil {
		return nil, nil, fmt.Errorf("determine primary region: %w", err)
	}

	var groups []replicaGroup
	if replicate {
		apiKey := os.Getenv("IBMCLOUD_API_KEY")
		tok, err := iamToken(apiKey)
		if err != nil {
			return nil, nil, fmt.Errorf("get IAM token for region listing: %w", err)
		}
		replicaRegions, err := otherVPCRegions(primaryRegion, tok)
		if err != nil {
			return nil, nil, fmt.Errorf("list IBM VPC replica regions: %w", err)
		}
		imageName := imageNameOutput.Value.(string)
		primaryGeo := ibmVPCRegionGeo(primaryRegion)
		primaryBucket := bucketNameOutput.Value.(string)
		primaryCOSURI := cosURIOutput.Value.(string)
		groups = buildReplicaGroups(primaryRegion, primaryGeo, primaryBucket, primaryCOSURI, imageName, replicaRegions)
	}

	r := ibmRegisterRequest{
		imageName:            imageNameOutput.Value.(string),
		cosURI:               cosURIOutput.Value.(string),
		bucketName:           bucketNameOutput.Value.(string),
		primaryRegion:        primaryRegion,
		osSlug:               osSlugOutput.Value.(string),
		existingAuthPolicyID: existingPolicyID,
		replicaGroups:        groups,
	}
	return r.registerFunc, nil, nil
}

// buildReplicaGroups groups replica VPC regions by IBM COS geo so that each geo
// needs at most one COS bucket and one streaming copy. Regions in the same geo as
// the primary reuse the primary bucket (no copy). Regions with no geo get individual
// regional buckets.
func buildReplicaGroups(primaryRegion, primaryGeo, primaryBucket, primaryCOSURI, imageName string, replicaRegions []string) []replicaGroup {
	geoRegions := make(map[string][]string)
	var noGeoRegions []string

	for _, r := range replicaRegions {
		geo := ibmVPCRegionGeo(r)
		if geo == "" {
			noGeoRegions = append(noGeoRegions, r)
		} else {
			geoRegions[geo] = append(geoRegions[geo], r)
		}
	}

	var groups []replicaGroup

	// Same-geo replicas reuse the primary COS bucket — no copy needed.
	if sameGeoRegions, ok := geoRegions[primaryGeo]; ok && primaryGeo != "" {
		groups = append(groups, replicaGroup{
			cosURI:    primaryCOSURI,
			bucket:    primaryBucket,
			endpoint:  ibmCOSBucketEndpoint(primaryRegion),
			regions:   sameGeoRegions,
			needsCopy: false,
		})
		delete(geoRegions, primaryGeo)
	}

	// Other-geo replicas: one cross-region bucket per geo, one streaming copy.
	for geo, regions := range geoRegions {
		bucket := stableBucketNameForGeo(imageName, geo)
		groups = append(groups, replicaGroup{
			cosURI:             fmt.Sprintf("cos://%s/%s/disk.qcow2", geo, bucket),
			bucket:             bucket,
			endpoint:           ibmCOSGeoEndpoints[geo],
			locationConstraint: ibmCOSGeoConstraints[geo],
			regions:            regions,
			needsCopy:          true,
		})
	}

	// No-geo replicas: individual regional bucket per region.
	for _, r := range noGeoRegions {
		bucket := stableBucketNameForRegion(imageName, r)
		groups = append(groups, replicaGroup{
			cosURI:             fmt.Sprintf("cos://%s/%s/disk.qcow2", r, bucket),
			bucket:             bucket,
			endpoint:           ibmCOSEndpoint(r),
			locationConstraint: ibmCOSLocationConstraint(r),
			regions:            []string{r},
			needsCopy:          true,
		})
	}

	return groups
}

type ibmRegisterRequest struct {
	imageName            string
	cosURI               string   // primary region COS URI
	bucketName           string   // primary COS bucket name
	primaryRegion        string   // primary VPC region (streaming source for copies)
	osSlug               string
	existingAuthPolicyID string
	replicaGroups        []replicaGroup
}

func (r *ibmRegisterRequest) registerFunc(ctx *pulumi.Context) error {
	name := sanitizeImageName(r.imageName)

	providerOpts := []pulumi.ResourceOption{}
	if v := ibmcloudProviderVersion(); v != "" {
		providerOpts = append(providerOpts, pulumi.Version(v))
	}
	ibmcloudProvider, err := ibmcloud.NewProvider(ctx, "ibmcloud", &ibmcloud.ProviderArgs{
		IbmcloudApiKey: pulumi.StringPtr(os.Getenv("IBMCLOUD_API_KEY")),
		Region:         pulumi.StringPtr(r.primaryRegion),
	}, providerOpts...)
	if err != nil {
		return fmt.Errorf("create IBM Cloud provider: %w", err)
	}
	prov := pulumi.Provider(ibmcloudProvider)

	rgName := sanitizeImageName(context.ProjectName())
	if len(rgName) > 40 {
		rgName = strings.TrimRight(rgName[:40], "-")
	}
	rg, err := ibmcloud.NewResourceGroup(ctx, "resourceGroup", &ibmcloud.ResourceGroupArgs{
		Name: pulumi.String(rgName),
	}, prov)
	if err != nil {
		return err
	}

	authPolicyOpts := []pulumi.ResourceOption{prov, pulumi.DependsOn([]pulumi.Resource{rg})}
	if r.existingAuthPolicyID != "" {
		authPolicyOpts = append(authPolicyOpts, pulumi.Import(pulumi.ID(r.existingAuthPolicyID)))
	}
	authPolicy, err := ibmcloud.NewIamAuthorizationPolicy(ctx, "vpcCosAuth", &ibmcloud.IamAuthorizationPolicyArgs{
		SourceServiceName:  pulumi.String("is"),
		SourceResourceType: pulumi.String("image"),
		TargetServiceName:  pulumi.String("cloud-object-storage"),
		Roles:              pulumi.StringArray{pulumi.String("Reader")},
	}, authPolicyOpts...)
	if err != nil {
		return err
	}

	image, err := ibmcloud.NewIsImage(ctx, "image", &ibmcloud.IsImageArgs{
		Name:            pulumi.String(name),
		Href:            pulumi.String(r.cosURI),
		OperatingSystem: pulumi.String(r.osSlug),
		ResourceGroup:   rg.ID().ToStringPtrOutput(),
		AllowedUse: ibmcloud.IsImageAllowedUsePtrInput(&ibmcloud.IsImageAllowedUseArgs{
			BareMetalServer: pulumi.StringPtr("true"),
			Instance:        pulumi.StringPtr("true"),
		}),
	},
		prov,
		pulumi.DependsOn([]pulumi.Resource{rg, authPolicy}),
		pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "6h",
			Update: "2h",
			Delete: "30m",
		}))
	if err != nil {
		return err
	}

	// Replicate to additional regions grouped by COS geo.
	// Cross-region geo groups share one bucket (no copy for same-geo; one copy for other geos).
	// Regions with no geo get individual regional buckets.
	var allReplicaRegions []string
	for _, group := range r.replicaGroups {
		if err := r.replicateGroup(ctx, group, rg, authPolicy, providerOpts); err != nil {
			return err
		}
		allReplicaRegions = append(allReplicaRegions, group.regions...)
	}
	if len(allReplicaRegions) > 0 {
		ctx.Export("replicaRegions", pulumi.String(strings.Join(allReplicaRegions, ",")))
	}

	return r.registerCatalogSharing(ctx, prov, image)
}

// replicateGroup handles replication for one geo group: optionally creates a COS
// bucket and streams the disk, then creates VPC images in all regions of the group.
func (r *ibmRegisterRequest) replicateGroup(
	ctx *pulumi.Context,
	group replicaGroup,
	rg *ibmcloud.ResourceGroup,
	authPolicy *ibmcloud.IamAuthorizationPolicy,
	providerOpts []pulumi.ResourceOption,
) error {
	var uploadDep pulumi.Resource

	if group.needsCopy {
		bucket, err := local.NewCommand(ctx, "cosReplicaBucket-"+group.bucket, &local.CommandArgs{
			Create: pulumi.String(fmt.Sprintf(
				"AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
					" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
					" aws s3api create-bucket --bucket %s"+
					" --create-bucket-configuration LocationConstraint=%s"+
					" || AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
					" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
					" aws s3api head-bucket --bucket %s",
				group.endpoint, group.bucket, group.locationConstraint, group.endpoint, group.bucket)),
			Delete: pulumi.String(fmt.Sprintf(
				"AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
					" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
					" aws s3 rb s3://%s --force 2>/dev/null || true",
				group.endpoint, group.bucket)),
		})
		if err != nil {
			return fmt.Errorf("create COS bucket %s: %w", group.bucket, err)
		}

		srcEndpoint := ibmCOSBucketEndpoint(r.primaryRegion)
		// 128 MB chunks reduce multipart requests from ~5000 to ~312 for a 40 GB image.
		// 32 concurrent requests maximises TCP window utilisation on cross-region links.
		copyCreate := fmt.Sprintf(
			"set -o pipefail;"+
				" export AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY;"+
				" export AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY;"+
				" export AWS_S3_MULTIPART_CHUNKSIZE=128MB;"+
				" export AWS_MAX_CONCURRENT_REQUESTS=32;"+
				" for i in 1 2 3; do"+
				"   src_size=$(AWS_ENDPOINT_URL_S3=%s aws s3api head-object"+
				" --bucket %s --key disk.qcow2 --query ContentLength --output text) &&"+
				"   AWS_ENDPOINT_URL_S3=%s aws s3 cp s3://%s/disk.qcow2 -"+
				" --no-progress --cli-read-timeout 0 |"+
				"   AWS_ENDPOINT_URL_S3=%s aws s3 cp - s3://%s/disk.qcow2"+
				" --expected-size \"$src_size\" --no-progress --cli-read-timeout 0 &&"+
				"   dst_size=$(AWS_ENDPOINT_URL_S3=%s aws s3api head-object"+
				" --bucket %s --key disk.qcow2 --query ContentLength --output text) &&"+
				"   [ \"$src_size\" -eq \"$dst_size\" ] &&"+
				"   echo \"replicaUpload %s: success ($src_size bytes)\" &&"+
				"   break;"+
				"   echo \"replicaUpload %s: attempt $i of 3 failed, retrying in 60s...\";"+
				"   [ $i -lt 3 ] && sleep 60;"+
				" done",
			srcEndpoint, r.bucketName,
			srcEndpoint, r.bucketName,
			group.endpoint, group.bucket,
			group.endpoint, group.bucket,
			group.bucket,
			group.bucket,
		)
		copyDelete := fmt.Sprintf(
			"AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
				" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
				" aws s3 rm s3://%s/disk.qcow2 2>/dev/null || true",
			group.endpoint, group.bucket,
		)
		upload, err := local.NewCommand(ctx, "replicaUpload-"+group.bucket, &local.CommandArgs{
			Create: pulumi.String(copyCreate),
			Delete: pulumi.String(copyDelete),
		}, pulumi.DependsOn([]pulumi.Resource{bucket}),
			pulumi.Timeouts(&pulumi.CustomTimeouts{
				Create: "6h",
				Update: "6h",
				Delete: "90m",
			}))
		if err != nil {
			return fmt.Errorf("stream disk to %s: %w", group.bucket, err)
		}
		uploadDep = upload
	}

	// Create a VPC image in each region of this group, all pointing at the same COS URI.
	for _, targetRegion := range group.regions {
		targetProvider, err := ibmcloud.NewProvider(ctx, "ibmcloud-"+targetRegion, &ibmcloud.ProviderArgs{
			IbmcloudApiKey: pulumi.StringPtr(os.Getenv("IBMCLOUD_API_KEY")),
			Region:         pulumi.StringPtr(targetRegion),
		}, providerOpts...)
		if err != nil {
			return fmt.Errorf("create IBM Cloud provider for %s: %w", targetRegion, err)
		}

		deps := []pulumi.Resource{rg, authPolicy}
		if uploadDep != nil {
			deps = append(deps, uploadDep)
		}

		replicaImage, err := ibmcloud.NewIsImage(ctx, "image-"+targetRegion, &ibmcloud.IsImageArgs{
			Name:            pulumi.String(sanitizeImageName(r.imageName)),
			Href:            pulumi.String(group.cosURI),
			OperatingSystem: pulumi.String(r.osSlug),
			ResourceGroup:   rg.ID().ToStringPtrOutput(),
			AllowedUse: ibmcloud.IsImageAllowedUsePtrInput(&ibmcloud.IsImageAllowedUseArgs{
				BareMetalServer: pulumi.StringPtr("true"),
				Instance:        pulumi.StringPtr("true"),
			}),
		},
			pulumi.Provider(targetProvider),
			pulumi.DependsOn(deps),
			pulumi.Timeouts(&pulumi.CustomTimeouts{
				Create: "6h",
				Update: "2h",
				Delete: "30m",
			}))
		if err != nil {
			return fmt.Errorf("create replica VPC image in %s: %w", targetRegion, err)
		}

		ctx.Export("vpcImageID-"+targetRegion, replicaImage.ID())
		ctx.Export("vpcImageName-"+targetRegion, pulumi.String(r.imageName))
	}

	return nil
}

func (r *ibmRegisterRequest) registerCatalogSharing(ctx *pulumi.Context, prov pulumi.ResourceOption, image *ibmcloud.IsImage) error {
	catalog, err := ibmcloud.NewCmCatalog(ctx, "catalog", &ibmcloud.CmCatalogArgs{
		Label:            pulumi.StringPtr(context.ProjectName()),
		ShortDescription: pulumi.StringPtr("Private catalog for cloud-importer VPC images"),
	}, prov, pulumi.IgnoreChanges([]string{"kind"}))
	if err != nil {
		return err
	}

	ctx.Export("catalogID", catalog.ID())
	ctx.Export("catalogName", pulumi.String(context.ProjectName()))
	ctx.Export("vpcImageID", image.ID())
	ctx.Export("vpcImageName", pulumi.String(r.imageName))
	ctx.Export(outOfferingName, pulumi.String(sanitizeImageName(r.imageName)))
	return nil
}
