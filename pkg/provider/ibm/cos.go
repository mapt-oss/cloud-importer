package ibm

import (
	"fmt"
	"os"

	ibmcloud "github.com/mapt-oss/pulumi-ibmcloud/sdk/go/ibmcloud"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var (
	outImageName  = "imagename"
	outArch       = "arch"
	outCOSURI     = "cosuri"
	outBucketName = "bucketname"
	outOSSlug     = "osslug"
)

// bucketEphemeral creates a temporary regional IBM COS bucket for image upload.
func bucketEphemeral(ctx *pulumi.Context, bucketName, cosInstanceID, region string) (*ibmcloud.CosBucket, error) {
	return ibmcloud.NewCosBucket(ctx, "cosEphemeralBucket", &ibmcloud.CosBucketArgs{
		BucketName:         pulumi.String(bucketName),
		ResourceInstanceId: pulumi.String(cosInstanceID),
		RegionLocation:     pulumi.String(region),
		StorageClass:       pulumi.String("standard"),
	})
}

// emptyBucketOnDestroy registers a delete-only command that empties the bucket
// before Pulumi tries to delete it (non-empty buckets cannot be destroyed).
func emptyBucketOnDestroy(ctx *pulumi.Context, bucketName, region string, bucket *ibmcloud.CosBucket) (pulumi.Resource, error) {
	endpoint := ibmCOSEndpoint(region)
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
	endpoint := ibmCOSEndpoint(region)
	uploadCmd := fmt.Sprintf(
		"AWS_ENDPOINT_URL_S3=%s AWS_ACCESS_KEY_ID=$IBMCLOUD_COS_ACCESS_KEY"+
			" AWS_SECRET_ACCESS_KEY=$IBMCLOUD_COS_SECRET_KEY"+
			" aws s3 cp %s s3://%s/%s --only-show-error",
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

// ibmCOSEndpoint returns the IBM COS regional S3-compatible endpoint.
// Override with IBMCLOUD_COS_ENDPOINT for private/custom endpoints.
func ibmCOSEndpoint(region string) string {
	if ep := os.Getenv("IBMCLOUD_COS_ENDPOINT"); ep != "" {
		return ep
	}
	return fmt.Sprintf("https://s3.%s.cloud-object-storage.appdomain.cloud", region)
}

// DeleteLocks is a no-op for IBM: use a file:/// local backend, or an
// s3:// backend pointed at IBM COS via AWS_ENDPOINT_URL_S3 with HMAC credentials
// (in which case the AWS provider's DeleteLocks will be invoked instead).
func DeleteLocks(_ string) {}

// CleanupState is a no-op for IBM (see DeleteLocks).
func CleanupState(_ string) {}
