package ibm

import (
	"fmt"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const rhelaiArch = "x86_64"

type rhelaiEphemeralRequest struct {
	qcow2ImageFilePath string
	imageName          string
}

func (p *ibmProvider) RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc {
	r := rhelaiEphemeralRequest{imageFilePath, imageName}
	return r.rhelaiEphemeralRunFunc
}

func (r rhelaiEphemeralRequest) rhelaiEphemeralRunFunc(ctx *pulumi.Context) error {
	if filepath.Ext(r.qcow2ImageFilePath) != ".qcow2" {
		return fmt.Errorf("--image-path must be a qcow2 disk image (*.qcow2); got %q", r.qcow2ImageFilePath)
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

	_, err = uploadDisk(ctx, r.qcow2ImageFilePath, "disk.qcow2", bucketName, region,
		[]pulumi.Resource{bucket})
	return err
}
