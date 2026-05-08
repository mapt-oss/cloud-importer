package gcp

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func randomID() *string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	id := fmt.Sprintf("cloud-importer-%x", b)
	return &id
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
	return storage.NewBucket(ctx, "gcsBucket", &storage.BucketArgs{
		Name:                     pulumi.String(*bucketName),
		ForceDestroy:             pulumi.Bool(true),
		UniformBucketLevelAccess: pulumi.Bool(true),
	})
}

// compressAndUpload packages rawFilePath as disk.raw inside a tar.gz and
// uploads it to gs://bucketName/disk.raw.tar.gz using gsutil.
func compressAndUpload(ctx *pulumi.Context, rawFilePath, bucketName *string, deps []pulumi.Resource) (pulumi.Resource, error) {
	tarPath := fmt.Sprintf("/tmp/%s-disk.raw.tar.gz", *bucketName)
	gcsURI := fmt.Sprintf("gs://%s/disk.raw.tar.gz", *bucketName)

	// Create a temp dir, copy the file as disk.raw, tar it, upload, then clean up.
	createCmd := fmt.Sprintf(
		"TMPDIR=$(mktemp -d) && cp %s $TMPDIR/disk.raw && "+
			"tar czf %s -C $TMPDIR disk.raw && rm -rf $TMPDIR && "+
			"gsutil cp %s %s && rm -f %s",
		*rawFilePath, tarPath, tarPath, gcsURI, tarPath)
	deleteCmd := fmt.Sprintf("gsutil rm -f %s || true", gcsURI)

	return local.NewCommand(ctx, "uploadGCS", &local.CommandArgs{
		Create: pulumi.String(createCmd),
		Delete: pulumi.String(deleteCmd),
	},
		pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Update: "6h", Delete: "30m"}),
		pulumi.DependsOn(deps))
}
