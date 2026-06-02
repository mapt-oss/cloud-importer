package gcp

import (
	"fmt"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/util"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// stableBucketName derives a deterministic GCS bucket name from the image name
// so that retries with the same --image-name reuse the existing bucket rather
// than triggering a Pulumi replace + re-upload (mirrors Azure's approach).
// The "ci-" prefix ensures the name never starts with a digit.
// The base is capped at 60 chars so the total stays within the 63-char GCS limit.
func stableBucketName(imageName string) *string {
	base := util.SanitizeBucketName(imageName)
	if len(base) > 60 {
		base = strings.TrimRight(base[:60], "-")
	}
	name := "ci-" + base
	return &name
}

func bucketEphemeral(ctx *pulumi.Context, bucketName *string) (*storage.Bucket, error) {
	location, err := sourceHostingPlace()
	if err != nil {
		return nil, err
	}
	return storage.NewBucket(ctx, "gcsBucket", &storage.BucketArgs{
		Name:                     pulumi.String(*bucketName),
		Location:                 pulumi.String(*location),
		ForceDestroy:             pulumi.Bool(true),
		UniformBucketLevelAccess: pulumi.Bool(true),
	})
}

// compressToLocal pads rawFilePath to a 1 GiB boundary and packages it as
// disk.raw inside a GNU-format tar.gz at tarPath. No credentials required.
// The delete command removes the local tar.gz.
func compressToLocal(ctx *pulumi.Context, rawFilePath, tarPath *string, deps []pulumi.Resource) (pulumi.Resource, error) {
	// GCP requires disk.raw size to be a multiple of 1GiB; pad with zeros if needed.
	padCmd := "SIZE=$(wc -c < \"$WORKDIR/disk.raw\" | tr -d ' ') && " +
		"NEXT_GIB=$(( (SIZE + 1073741823) / 1073741824 * 1073741824 )) && " +
		"truncate -s $NEXT_GIB \"$WORKDIR/disk.raw\""

	// GCP requires GNU or OLDGNU tar format. macOS BSD tar uses "gnutar" as the
	// format name; Linux GNU tar uses "gnu". Detect at runtime.
	tarFmtCmd := "if tar --version 2>&1 | grep -q 'GNU tar'; then " +
		"TAR_FORMAT=\"--format=gnu\"; " +
		"else TAR_FORMAT=\"--format=gnutar\"; fi"

	createCmd := fmt.Sprintf(
		// WORKDIR is created alongside the source image (on the host-mounted volume)
		// to avoid filling the container overlay filesystem with a large copy.
		"WORKDIR=$(mktemp -d $(dirname %s)/gcp-tmp.XXXXXX) && "+
			"trap 'rm -rf \"$WORKDIR\"' EXIT && cp %s $WORKDIR/disk.raw && "+
			"%s && %s && "+
			"tar $TAR_FORMAT -czf %s -C $WORKDIR disk.raw",
		*rawFilePath, *rawFilePath, padCmd, tarFmtCmd, *tarPath)
	deleteCmd := fmt.Sprintf("rm -f %s", *tarPath)

	return local.NewCommand(ctx, "compressGCS", &local.CommandArgs{
		Create: pulumi.String(createCmd),
		Delete: pulumi.String(deleteCmd),
	},
		pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Delete: "30m"}),
		pulumi.DependsOn(deps))
}

// uploadToGCS streams tarPath to gs://bucketName/disk.raw.tar.gz via a
// local.Command that invokes "cloud-importer gcs-upload". The gcs-upload
// subcommand uses the GCS JSON API resumable upload which streams the file
// in chunks rather than loading it entirely into memory, making it safe for
// large disk images.
func uploadToGCS(ctx *pulumi.Context, bucketName, tarPath string, deps []pulumi.Resource) (pulumi.Resource, error) {
	uploadCmd := fmt.Sprintf(
		"cloud-importer gcs-upload --bucket %s --object disk.raw.tar.gz --source %s",
		bucketName, tarPath)

	return local.NewCommand(ctx, "uploadGCS", &local.CommandArgs{
		Create: pulumi.String(uploadCmd),
		// GCS object is cleaned up by the bucket's ForceDestroy on destroy.
		Delete: pulumi.String("true"),
	},
		pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Delete: "30m"}),
		pulumi.DependsOn(deps))
}
