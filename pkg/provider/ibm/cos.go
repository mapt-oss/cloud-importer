package ibm

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var (
	outImageName    = "imagename"
	outArch         = "arch"
	outCOSURI       = "cosuri"
	outBucketName   = "bucketname"
	outOSSlug       = "osslug"
	outOfferingName = "offeringname"
)

// vpcRegionGeo maps IBM VPC regions to IBM COS cross-region geos (us/eu/ap).
// Regions absent from this map have no cross-region COS geo and require individual
// regional COS buckets.
var vpcRegionGeo = map[string]string{
	"us-south": "us",
	"us-east":  "us",
	"eu-gb":    "eu",
	"eu-de":    "eu",
	"eu-es":    "eu",
	"jp-tok":   "ap",
	"jp-osa":   "ap",
	"au-syd":   "ap",
}

// ibmCOSGeoEndpoints maps IBM COS cross-region geo names to their public endpoints.
var ibmCOSGeoEndpoints = map[string]string{
	"us": "https://s3.us.cloud-object-storage.appdomain.cloud",
	"eu": "https://s3.eu.cloud-object-storage.appdomain.cloud",
	"ap": "https://s3.ap.cloud-object-storage.appdomain.cloud",
}

// ibmCOSGeoConstraints maps IBM COS cross-region geo names to the LocationConstraints
// required when creating a cross-region bucket.
var ibmCOSGeoConstraints = map[string]string{
	"us": "us-standard",
	"eu": "eu-standard",
	"ap": "ap-standard",
}

// ibmCOSSupportedRegions is the set of IBM VPC regions that have IBM COS
// regional endpoints (confirmed from IBM docs, last updated Feb 2026).
var ibmCOSSupportedRegions = map[string]bool{
	"us-south": true,
	"us-east":  true,
	"eu-gb":    true,
	"eu-de":    true,
	"eu-es":    true,
	"jp-tok":   true,
	"jp-osa":   true,
	"au-syd":   true,
	"ca-tor":   true,
	"ca-mon":   true,
	"br-sao":   true,
	"in-che":   true,
	"in-mum":   true,
}

// ibmVPCRegionGeo returns the IBM COS cross-region geo for a VPC region,
// or "" if the region has no cross-region COS geo.
func ibmVPCRegionGeo(region string) string {
	return vpcRegionGeo[region]
}

// ibmCOSBucketEndpoint returns the S3 endpoint for the COS bucket that stores
// data for the given VPC region. For regions with a cross-region geo the geo
// endpoint is returned; otherwise the regional endpoint is returned.
// Override with IBMCLOUD_COS_ENDPOINT for private/custom endpoints.
func ibmCOSBucketEndpoint(region string) string {
	if ep := os.Getenv("IBMCLOUD_COS_ENDPOINT"); ep != "" {
		return ep
	}
	if geo := ibmVPCRegionGeo(region); geo != "" {
		return ibmCOSGeoEndpoints[geo]
	}
	return fmt.Sprintf("https://s3.%s.cloud-object-storage.appdomain.cloud", region)
}

// ibmCOSEndpoint returns the IBM COS regional S3-compatible endpoint for a
// specific region, ignoring any cross-region geo. Used for replica regional buckets.
// Override with IBMCLOUD_COS_ENDPOINT for private/custom endpoints.
func ibmCOSEndpoint(region string) string {
	if ep := os.Getenv("IBMCLOUD_COS_ENDPOINT"); ep != "" {
		return ep
	}
	return fmt.Sprintf("https://s3.%s.cloud-object-storage.appdomain.cloud", region)
}

// ibmCOSBucketConstraint returns the LocationConstraint for bucket creation for
// the given VPC region. Cross-region geos use "us-standard" etc.; regional
// buckets use "<region>-standard".
func ibmCOSBucketConstraint(region string) string {
	if geo := ibmVPCRegionGeo(region); geo != "" {
		return ibmCOSGeoConstraints[geo]
	}
	return region + "-standard"
}

// ibmCOSLocationConstraint returns the regional location constraint ("<region>-standard").
// Used for replica buckets in regions that have no cross-region geo.
func ibmCOSLocationConstraint(region string) string {
	return region + "-standard"
}

// ibmCOSURI returns the cos:// URI for a VPC image import.
// For VPC regions with a cross-region geo the geo name is used (cos://us/...)
// so IBM VPC can import from any region in that geo without a per-region copy.
// For regions with no geo the regional URI is used (cos://region/...).
func ibmCOSURI(region, bucket, object string) string {
	if geo := ibmVPCRegionGeo(region); geo != "" {
		return fmt.Sprintf("cos://%s/%s/%s", geo, bucket, object)
	}
	return fmt.Sprintf("cos://%s/%s/%s", region, bucket, object)
}

// bucketEphemeral creates the primary COS bucket for image upload.
// For regions with a cross-region geo a cross-region bucket is created so that
// all VPC regions in that geo can import from it without a per-region copy.
func bucketEphemeral(ctx *pulumi.Context, bucketName, region string) (pulumi.Resource, error) {
	endpoint := ibmCOSBucketEndpoint(region)
	constraint := ibmCOSBucketConstraint(region)
	return local.NewCommand(ctx, "cosEphemeralBucket", &local.CommandArgs{
		Create: pulumi.String(fmt.Sprintf(
			"AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
				" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
				" aws s3api create-bucket --bucket %s"+
				" --create-bucket-configuration LocationConstraint=%s"+
				" || AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
				" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
				" aws s3api head-bucket --bucket %s",
			endpoint, bucketName, constraint, endpoint, bucketName)),
		Delete: pulumi.String(fmt.Sprintf(
			"AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
				" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
				" aws s3 rb s3://%s 2>/dev/null || true",
			endpoint, bucketName)),
	})
}

// emptyBucketOnDestroy registers a delete-only command that empties the bucket
// before Pulumi tries to delete it (non-empty buckets cannot be destroyed).
func emptyBucketOnDestroy(ctx *pulumi.Context, bucketName, region string, bucket pulumi.Resource) (pulumi.Resource, error) {
	endpoint := ibmCOSBucketEndpoint(region)
	return local.NewCommand(ctx, "emptyBucket", &local.CommandArgs{
		Delete: pulumi.String(fmt.Sprintf(
			"AWS_ENDPOINT_URL_S3=%s aws s3 rm s3://%s/ --recursive --only-show-error 2>/dev/null || true",
			endpoint, bucketName)),
	}, pulumi.DependsOn([]pulumi.Resource{bucket}))
}

// uploadDisk uploads imageFilePath to s3://<bucketName>/<objectKey> via the
// IBM COS S3-compatible API, using HMAC credentials from the environment.
// The caller must set IBMCLOUD_COS_ACCESS_KEY and IBMCLOUD_COS_SECRET_KEY.
func uploadDisk(ctx *pulumi.Context, imageFilePath, objectKey, bucketName, region string,
	dependencies []pulumi.Resource) (pulumi.Resource, error) {
	endpoint := ibmCOSBucketEndpoint(region)
	// IBM COS cross-region buckets propagate across geo nodes with eventual
	// consistency; poll head-bucket for up to 5 minutes before uploading.
	uploadCmd := fmt.Sprintf(
		"for i in $(seq 1 20); do"+
			" AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
			" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
			" aws s3api head-bucket --bucket %s 2>/dev/null && break;"+
			" echo \"waiting for bucket %s to be available ($i/20)...\";"+
			" sleep 15;"+
			" done &&"+
			" AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
			" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
			" aws s3 cp %s s3://%s/%s --only-show-error",
		endpoint, bucketName, bucketName,
		endpoint, imageFilePath, bucketName, objectKey)
	deleteCmd := fmt.Sprintf(
		"AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
			" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
			" aws s3 rm s3://%s/%s --only-show-error",
		endpoint, bucketName, objectKey)
	return local.NewCommand(ctx, "upload", &local.CommandArgs{
		Create: pulumi.String(uploadCmd),
		Delete: pulumi.String(deleteCmd),
	}, pulumi.Timeouts(&pulumi.CustomTimeouts{
		Create: "6h",
		Update: "6h",
		Delete: "90m",
	}), pulumi.DependsOn(dependencies))
}

// DeleteLocks is a no-op for IBM: use a file:/// local backend, or an
// s3:// backend pointed at IBM COS via AWS_ENDPOINT_URL_S3 with HMAC credentials
// (in which case the AWS provider's DeleteLocks will be invoked instead).
func DeleteLocks(_ string) {}

// CleanupState is a no-op for IBM (see DeleteLocks).
func CleanupState(_ string) {}
