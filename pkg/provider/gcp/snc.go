package gcp

import (
	"fmt"

	"github.com/mapt-oss/cloud-importer/pkg/util/bundle"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var gcpBundleArch = map[string]*bundle.BundleArch{
	"x86_64": &bundle.AMD64,
	"arm64":  &bundle.ARM64,
}

type sncEphemeralRequest struct {
	bundleURI string
	shasumURI string
	arch      string
}

func (p *gcpProvider) SNCEphemeral(bundleURI, shasumURI, arch string) pulumi.RunFunc {
	r := sncEphemeralRequest{bundleURI, shasumURI, arch}
	return r.sncEphemeralRunFunc
}

func (r sncEphemeralRequest) sncEphemeralRunFunc(ctx *pulumi.Context) error {
	bundleArch, ok := gcpBundleArch[r.arch]
	if !ok {
		return fmt.Errorf("unsupported arch %q for GCP: must be x86_64 or arm64", r.arch)
	}

	baseName, err := bundle.GetDescription(r.bundleURI, bundleArch)
	if err != nil {
		return err
	}
	imageName := fmt.Sprintf("%s-%s", *baseName, r.arch)
	ctx.Export(outImageName, pulumi.String(imageName))
	ctx.Export(outGCPArch, pulumi.String(r.arch))

	extractExecution, err := bundle.Extract(ctx, imageName, r.bundleURI, r.shasumURI, "gcp")
	if err != nil {
		return err
	}

	bucketName := stableBucketName(imageName)
	bucket, err := bucketEphemeral(ctx, bucketName)
	if err != nil {
		return err
	}

	gcsURI := fmt.Sprintf("gs://%s/disk.raw.tar.gz", *bucketName)
	ctx.Export(outGCSURI, pulumi.String(gcsURI))

	rawPath := bundle.ExtractedRAWDiskFileName
	_, err = compressAndUpload(ctx, &rawPath, bucketName, []pulumi.Resource{bucket, extractExecution})
	return err
}
