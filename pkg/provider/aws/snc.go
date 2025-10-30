package aws

import (
	"fmt"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/bundle"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type sncEphemeralRequest struct {
	bundleURI string
	shasumURI string
	arch      *AMIArch
}

var (
	amiArch = map[string]*AMIArch{
		"x86_64": &X86,
		"arm64":  &ARM64,
	}
	bundleArch = map[*AMIArch]*bundle.BundleArch{
		&X86:   &bundle.AMD64,
		&ARM64: &bundle.ARM64}
)

func (a *aws) SNCEphemeral(bundleURI, shasumURI, arch string) pulumi.RunFunc {
	r := sncEphemeralRequest{
		bundleURI: bundleURI,
		shasumURI: shasumURI,
		arch:      amiArch[arch]}
	return r.sncEphemeralRunFunc
}

func (r sncEphemeralRequest) sncEphemeralRunFunc(ctx *pulumi.Context) error {
	extractExecution, err := bundle.Extract(ctx, r.bundleURI, r.shasumURI, "aws")
	if err != nil {
		return err
	}
	amiBaseName, err := bundle.GetDescription(r.bundleURI, bundleArch[r.arch])
	if err != nil {
		return err
	}
	arch := string(*r.arch)
	ctx.Export(outAMIName,
		pulumi.String(
			fmt.Sprintf("%s-%s",
				*amiBaseName, arch)))
	ctx.Export(outAMIArch, pulumi.String(arch))
	bucketName := randomID()
	_, err = bucketEphemeral(ctx, bucketName)
	if err != nil {
		return err
	}
	ctx.Export(outBucketName, pulumi.String(*bucketName))
	ro, _, err := createVMIEmportExportRole(ctx, bucketName)
	if err != nil {
		return err
	}
	ctx.Export(outRoleName, ro.RoleName)
	_, err = uploadDisk(ctx, &bundle.ExtractedRAWDiskFileName, bucketName,
		[]pulumi.Resource{ro, extractExecution})
	return err
}
