package gcp

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var (
	outImageName = "imagename"
	outGCSURI    = "gcsuri"
	outGCPArch   = "arch"
)

const rhelaiArch = "X86_64"

type rhelaiEphemeralRequest struct {
	rawImageFilePath string
	imageName        string
}

// DeriveEphemeralOutputs constructs the outputs that the ephemeral stack would
// have produced, using only the image name. The GCS URI is deterministic because
// GCP uses stable (non-random) bucket names derived from the image name.
// This allows --image-path to be omitted when updating an already-imported image.
func (p *gcpProvider) DeriveEphemeralOutputs(imageName string) auto.OutputMap {
	bucketName := stableBucketName(imageName)
	return auto.OutputMap{
		outImageName: auto.OutputValue{Value: imageName},
		outGCSURI:    auto.OutputValue{Value: fmt.Sprintf("gs://%s/disk.raw.tar.gz", *bucketName)},
		outGCPArch:   auto.OutputValue{Value: rhelaiArch},
	}
}

func (p *gcpProvider) RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc {
	r := rhelaiEphemeralRequest{imageFilePath, imageName}
	return r.rhelaiEphemeralRunFunc
}

func (r rhelaiEphemeralRequest) rhelaiEphemeralRunFunc(ctx *pulumi.Context) error {
	ctx.Export(outImageName, pulumi.String(r.imageName))
	ctx.Export(outGCPArch, pulumi.String(rhelaiArch))

	bucketName := stableBucketName(r.imageName)
	tarPath := fmt.Sprintf("/tmp/%s-disk.raw.tar.gz", *bucketName)

	ctx.Export(outGCSURI, pulumi.String(fmt.Sprintf("gs://%s/disk.raw.tar.gz", *bucketName)))

	bucket, err := bucketEphemeral(ctx, bucketName)
	if err != nil {
		return err
	}

	compressed, err := compressToLocal(ctx, &r.rawImageFilePath, &tarPath, []pulumi.Resource{bucket})
	if err != nil {
		return err
	}

	_, err = uploadBucketObject(ctx, bucket, tarPath, []pulumi.Resource{compressed})
	return err
}

