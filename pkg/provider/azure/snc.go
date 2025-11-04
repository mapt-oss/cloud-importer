package azure

import (
	"fmt"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/bundle"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	sncOffer     = "snc"
	sncPublisher = "openshift-local"
	sncSKU       = "openshift_local_snc"
)

type sncEphemeralRequest struct {
	bundleURI string
	shasumURI string
	arch      string
}

func (a *azureProvider) SNCEphemeral(bundleURI, shasumURI, arch string) pulumi.RunFunc {
	r := sncEphemeralRequest{
		bundleURI: bundleURI,
		shasumURI: shasumURI,
		arch:      arch,
	}
	return r.sncEphemeralRunFunc
}

func (r *sncEphemeralRequest) sncEphemeralRunFunc(ctx *pulumi.Context) error {
	ctx.Export(outOffer, pulumi.String(sncOffer))
	ctx.Export(outPublisher, pulumi.String(sncPublisher))
	ctx.Export(outSKU, pulumi.String(sncSKU))
	imageBaseName, err := bundle.GetDescription(r.bundleURI, nil)
	if err != nil {
		return err
	}
	imageName := fmt.Sprintf("%s-%s", *imageBaseName, r.arch)
	ctx.Export(outName, pulumi.String(imageName))
	ctx.Export(outArch, pulumi.String(r.arch))
	extractCmd, err := bundle.Extract(ctx, r.bundleURI, r.shasumURI, "azure")
	if err != nil {
		return err
	}
	location, err := sourceHostingPlace()
	if err != nil {
		return err
	}
	container, sa, rg, err := storageAccount(ctx, location, &imageName)
	if err != nil {
		return err
	}
	ctx.Export(outServiceAccountId, sa.ID())
	blobName := "disk.vhd"
	sas := storageAccSAS(ctx, sa.Name, rg.Name)
	sasURL := pulumi.Sprintf(sasURLBase, sa.Name, container.Name, blobName, sas.AccountSasToken())
	blobURI := pulumi.Sprintf(blobURLBase, sa.Name, container.Name, blobName)
	ctx.Export(outBlobURI, blobURI)
	return uploadVHD(ctx, bundle.ExtractedVHDDiskFileName, sasURL, pulumi.DependsOn([]pulumi.Resource{container, extractCmd}))
}
