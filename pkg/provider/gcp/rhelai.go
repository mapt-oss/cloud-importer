package gcp

import (
	"fmt"

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

func (p *gcpProvider) RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc {
	r := rhelaiEphemeralRequest{imageFilePath, imageName}
	return r.rhelaiEphemeralRunFunc
}

func (r rhelaiEphemeralRequest) rhelaiEphemeralRunFunc(ctx *pulumi.Context) error {
	ctx.Export(outImageName, pulumi.String(r.imageName))
	ctx.Export(outGCPArch, pulumi.String(rhelaiArch))

	bucketName := stableBucketName(r.imageName)
	bucket, err := bucketEphemeral(ctx, bucketName)
	if err != nil {
		return err
	}

	gcsURI := fmt.Sprintf("gs://%s/disk.raw.tar.gz", *bucketName)
	ctx.Export(outGCSURI, pulumi.String(gcsURI))

	_, err = compressAndUpload(ctx, &r.rawImageFilePath, bucketName, []pulumi.Resource{bucket})
	return err
}
