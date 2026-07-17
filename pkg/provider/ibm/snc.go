package ibm

import (
	"fmt"

	"github.com/mapt-oss/cloud-importer/pkg/util/bundle"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
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
	cosID, err := envCosInstanceID()
	if err != nil {
		return err
	}
	osSlug, err := envVPCOperatingSystem()
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

	bucket, err := bucketEphemeral(ctx, bucketName, cosID, region)
	if err != nil {
		return err
	}
	if _, err = emptyBucketOnDestroy(ctx, bucketName, region, bucket); err != nil {
		return err
	}

	// extract.sh produces disk.raw regardless of provider; convert to qcow2
	// which is the format IBM VPC custom image import requires.
	extractExecution, err := bundle.Extract(ctx, imageName, r.bundleURI, r.shasumURI, "ibm")
	if err != nil {
		return err
	}
	convert, err := local.NewCommand(ctx, "convertToQcow2", &local.CommandArgs{
		Create: pulumi.String(fmt.Sprintf(
			"qemu-img convert -p -f raw -O qcow2 %s disk.qcow2 && rm -f %s",
			bundle.ExtractedRAWDiskFileName, bundle.ExtractedRAWDiskFileName)),
		Delete: pulumi.String("rm -f disk.qcow2"),
	}, pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "2h", Delete: "5m"}),
		pulumi.DependsOn([]pulumi.Resource{bucket, extractExecution}))
	if err != nil {
		return err
	}

	_, err = uploadDisk(ctx, "disk.qcow2", "disk.qcow2", bucketName, region,
		[]pulumi.Resource{bucket, convert})
	return err
}
