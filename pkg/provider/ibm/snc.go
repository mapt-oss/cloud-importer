package ibm

import (
	"fmt"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/util/bundle"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var ibmBundleArch = map[string]*bundle.BundleArch{
	"x86_64": &bundle.AMD64,
	"arm64":  &bundle.ARM64,
}

type sncEphemeralRequest struct {
	bundleURI string
	shasumURI string
	arch      string
}

// DeriveEphemeralOutputs reconstructs the ephemeral stack outputs from the
// image name alone so that a permission-only update can skip the upload entirely.
// The image name must follow the form produced by SNCEphemeral: "<description>-<arch>".
func (p *ibmProvider) DeriveEphemeralOutputs(imageName string) auto.OutputMap {
	arch := "x86_64"
	if i := strings.LastIndex(imageName, "-"); i >= 0 {
		if suffix := imageName[i+1:]; suffix == "x86_64" || suffix == "arm64" {
			arch = suffix
		}
	}
	osSlug, _ := sncVPCOperatingSystem(arch)
	region, _ := sourceRegion()
	bucketName := stableBucketName(imageName)
	return auto.OutputMap{
		outImageName:  auto.OutputValue{Value: imageName},
		outArch:       auto.OutputValue{Value: arch},
		outOSSlug:     auto.OutputValue{Value: osSlug},
		outBucketName: auto.OutputValue{Value: bucketName},
		outCOSURI:     auto.OutputValue{Value: fmt.Sprintf("cos://%s/%s/disk.qcow2", region, bucketName)},
	}
}

// ImageNameFromBundle derives the image name from the bundle URI without
// downloading the bundle, so the manager can detect an already-registered image
// and skip the ephemeral upload.
func (p *ibmProvider) ImageNameFromBundle(bundleURI, shasumURI, arch string) (string, error) {
	bundleArch, ok := ibmBundleArch[arch]
	if !ok {
		return "", fmt.Errorf("unsupported arch %q for IBM: must be x86_64 or arm64", arch)
	}
	baseName, err := bundle.GetDescription(bundleURI, bundleArch)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", *baseName, arch), nil
}

func (p *ibmProvider) SNCEphemeral(bundleURI, shasumURI, arch string) pulumi.RunFunc {
	r := sncEphemeralRequest{bundleURI, shasumURI, arch}
	return r.sncEphemeralRunFunc
}

func (r sncEphemeralRequest) sncEphemeralRunFunc(ctx *pulumi.Context) error {
	bundleArch, ok := ibmBundleArch[r.arch]
	if !ok {
		return fmt.Errorf("unsupported arch %q for IBM: must be x86_64 or arm64", r.arch)
	}

	region, err := sourceRegion()
	if err != nil {
		return err
	}
	osSlug, err := sncVPCOperatingSystem(r.arch)
	if err != nil {
		return err
	}

	baseName, err := bundle.GetDescription(r.bundleURI, bundleArch)
	if err != nil {
		return err
	}
	imageName := fmt.Sprintf("%s-%s", *baseName, r.arch)

	ctx.Export(outImageName, pulumi.String(imageName))
	ctx.Export(outArch, pulumi.String(r.arch))
	ctx.Export(outOSSlug, pulumi.String(osSlug))

	bucketName := stableBucketName(imageName)
	ctx.Export(outBucketName, pulumi.String(bucketName))
	ctx.Export(outCOSURI, pulumi.String(fmt.Sprintf("cos://%s/%s/disk.qcow2", region, bucketName)))

	bucket, err := bucketEphemeral(ctx, bucketName, region)
	if err != nil {
		return err
	}
	if _, err = emptyBucketOnDestroy(ctx, bucketName, region, bucket); err != nil {
		return err
	}

	// extract.sh converts disk.raw → disk.qcow2 for IBM (IBM VPC requires qcow2).
	extractExecution, err := bundle.Extract(ctx, imageName, r.bundleURI, r.shasumURI, "ibm")
	if err != nil {
		return err
	}

	_, err = uploadDisk(ctx, "disk.qcow2", "disk.qcow2", bucketName, region,
		[]pulumi.Resource{bucket, extractExecution})
	return err
}
