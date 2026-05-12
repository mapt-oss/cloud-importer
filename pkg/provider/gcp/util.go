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

// compressAndUpload packages rawFilePath as disk.raw inside a tar.gz and
// uploads it to gs://bucketName/disk.raw.tar.gz using gsutil.
func compressAndUpload(ctx *pulumi.Context, rawFilePath, bucketName *string, deps []pulumi.Resource) (pulumi.Resource, error) {
	tarPath := fmt.Sprintf("/tmp/%s-disk.raw.tar.gz", *bucketName)
	gcsURI := fmt.Sprintf("gs://%s/disk.raw.tar.gz", *bucketName)

	// Write GOOGLE_CREDENTIALS to a temp file so gcloud storage uses the same
	// service account as the Pulumi provider. gcloud storage reads
	// GOOGLE_APPLICATION_CREDENTIALS; if credentials are not set it falls
	// back to gcloud ADC.
	credSetup := "if [ -n \"${GOOGLE_CREDENTIALS}\" ]; then " +
		"_CREDS=$(mktemp) && printf '%s' \"${GOOGLE_CREDENTIALS}\" > \"$_CREDS\" && " +
		"export GOOGLE_APPLICATION_CREDENTIALS=\"$_CREDS\"; fi"

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
		"%s && TMPDIR=$(mktemp -d) && cp %s $TMPDIR/disk.raw && "+
			"%s && %s && "+
			"tar $TAR_FORMAT -czf %s -C $TMPDIR disk.raw && rm -rf $TMPDIR && "+
			"gcloud storage cp %s %s && rm -f %s ${_CREDS:-}",
		credSetup, *rawFilePath, padCmd, tarFmtCmd, tarPath, tarPath, gcsURI, tarPath)
	deleteCmd := fmt.Sprintf(
		"%s && gcloud storage rm %s || true && rm -f ${_CREDS:-}",
		credSetup, gcsURI)

	return local.NewCommand(ctx, "uploadGCS", &local.CommandArgs{
		Create: pulumi.String(createCmd),
		Delete: pulumi.String(deleteCmd),
	},
		pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Update: "6h", Delete: "30m"}),
		pulumi.DependsOn(deps))
}
