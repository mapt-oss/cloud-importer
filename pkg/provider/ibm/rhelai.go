package ibm

import (
	"fmt"
	"path/filepath"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const rhelaiArch = "x86_64"

type rhelaiEphemeralRequest struct {
	rawImageFilePath string
	imageName        string
}

func (p *ibmProvider) RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc {
	r := rhelaiEphemeralRequest{imageFilePath, imageName}
	return r.rhelaiEphemeralRunFunc
}

func (r rhelaiEphemeralRequest) rhelaiEphemeralRunFunc(ctx *pulumi.Context) error {
	if filepath.Ext(r.rawImageFilePath) != ".raw" {
		return fmt.Errorf("--image-path must be a raw disk image (*.raw); got %q", r.rawImageFilePath)
	}

	region, err := sourceRegion()
	if err != nil {
		return err
	}
	cosID, err := envCosInstanceID()
	if err != nil {
		return err
	}
	osSlug, err := envVPCOperatingSystem()
	if err != nil {
		return err
	}

	ctx.Export(outImageName, pulumi.String(r.imageName))
	ctx.Export(outArch, pulumi.String(rhelaiArch))
	ctx.Export(outOSSlug, pulumi.String(osSlug))

	bucketName := stableBucketName(r.imageName)
	ctx.Export(outBucketName, pulumi.String(bucketName))
	ctx.Export(outCOSURI, pulumi.String(fmt.Sprintf("cos://%s/%s/disk.qcow2", region, bucketName)))

	bucket, err := bucketEphemeral(ctx, bucketName, cosID, region)
	if err != nil {
		return err
	}
	if _, err = emptyBucketOnDestroy(ctx, bucketName, region, bucket); err != nil {
		return err
	}

	// IBM VPC does not support raw format; convert to qcow2 beside the source
	// image to avoid filling the container overlay filesystem.
	qcow2Path := filepath.Join(filepath.Dir(r.rawImageFilePath),
		bucketName+"-disk.qcow2")
	convert, err := local.NewCommand(ctx, "convertToQcow2", &local.CommandArgs{
		Create: pulumi.String(fmt.Sprintf(
			"qemu-img convert -p -f raw -O qcow2 %s %s",
			r.rawImageFilePath, qcow2Path)),
		Delete: pulumi.String(fmt.Sprintf("rm -f %s", qcow2Path)),
	}, pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "2h", Delete: "5m"}),
		pulumi.DependsOn([]pulumi.Resource{bucket}))
	if err != nil {
		return err
	}

	_, err = uploadDisk(ctx, qcow2Path, "disk.qcow2", bucketName, region,
		[]pulumi.Resource{bucket, convert})
	return err
}
