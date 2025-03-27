package aws

import (
	"fmt"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/bundle"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type openshiftRequest struct {
	bundleURL string
	shasumURL string
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

func (a *aws) OpenshiftLocal(bundleURL, shasumURL, arch string) (pulumi.RunFunc, error) {

	r := openshiftRequest{
		bundleURL,
		shasumURL,
		amiArch[arch]}
	return r.runFunc, nil
}

func (r openshiftRequest) runFunc(ctx *pulumi.Context) error {
	extractExecution, err := bundle.Extract(ctx, r.bundleURL, r.shasumURL)
	if err != nil {
		return err
	}
	amiBaseName, err := bundle.GetDescription(r.bundleURL, bundleArch[r.arch])
	if err != nil {
		return err
	}
	amiName := fmt.Sprintf("%s-%s", *amiBaseName, string(*r.arch))
	id := randomID()
	_, err = bucketEphemeral(ctx, id)
	if err != nil {
		return err
	}
	ro, _, err := createVMIEmportExportRole(ctx, id)
	if err != nil {
		return err
	}
	u, err := uploadDisk(ctx, &bundle.ExtractedDiskFileName, id,
		[]pulumi.Resource{ro, extractExecution})
	if err != nil {
		return err
	}
	arch := string(*r.arch)
	_, err = registerAMI(ctx, &amiName, &arch, id, ro, []pulumi.Resource{u})
	if err != nil {
		return err
	}
	return nil
}
