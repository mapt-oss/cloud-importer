package gcp

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// stableBucketName derives a deterministic GCS bucket name from the image name
// so that retries with the same --image-name reuse the existing bucket rather
// than triggering a Pulumi replace + re-upload (mirrors Azure's approach).
// The "ci-" prefix ensures the name never starts with a digit.
func stableBucketName(imageName string) *string {
	name := "ci-" + sanitizeImageName(imageName)
	if len(name) > 63 {
		name = name[:63]
	}
	name = strings.TrimRight(name, "-")
	return &name
}

// sanitizeImageName converts an image name to a GCP-compatible format:
// lowercase, only hyphens (no underscores/dots), max 63 chars.
func sanitizeImageName(name string) string {
	name = strings.ToLower(name)
	name = strings.NewReplacer("_", "-", ".", "-").Replace(name)
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.TrimRight(name, "-")
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
	padCmd := "SIZE=$(wc -c < \"$TMPDIR/disk.raw\" | tr -d ' ') && " +
		"NEXT_GIB=$(( (SIZE + 1073741823) / 1073741824 * 1073741824 )) && " +
		"truncate -s $NEXT_GIB \"$TMPDIR/disk.raw\""

	// GCP requires GNU or OLDGNU tar format. macOS BSD tar uses "gnutar" as the
	// format name; Linux GNU tar uses "gnu". Detect at runtime.
	tarFmtCmd := "if tar --version 2>&1 | grep -q 'GNU tar'; then " +
		"TAR_FORMAT=\"--format=gnu\"; " +
		"else TAR_FORMAT=\"--format=gnutar\"; fi"

	createCmd := fmt.Sprintf(
		"TMPDIR=$(mktemp -d) && cp %s $TMPDIR/disk.raw && "+
			"%s && %s && "+
			"tar $TAR_FORMAT -czf %s -C $TMPDIR disk.raw && rm -rf $TMPDIR",
		*rawFilePath, padCmd, tarFmtCmd, *tarPath)
	deleteCmd := fmt.Sprintf("rm -f %s", *tarPath)

	return local.NewCommand(ctx, "compressGCS", &local.CommandArgs{
		Create: pulumi.String(createCmd),
		Delete: pulumi.String(deleteCmd),
	},
		pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Delete: "30m"}),
		pulumi.DependsOn(deps))
}

// uploadBucketObject uploads tarPath to the GCS bucket as disk.raw.tar.gz
// using storage.NewBucketObject — credentials come from the Pulumi GCP
// provider config (gcp:credentials) with no temp file on disk.
func uploadBucketObject(ctx *pulumi.Context, bucket *storage.Bucket, tarPath string, deps []pulumi.Resource) (pulumi.Resource, error) {
	return storage.NewBucketObject(ctx, "uploadGCS", &storage.BucketObjectArgs{
		Bucket: bucket.Name,
		Name:   pulumi.String("disk.raw.tar.gz"),
		Source: pulumi.NewFileAsset(tarPath),
	},
		pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Update: "6h", Delete: "30m"}),
		pulumi.DependsOn(deps))
}
