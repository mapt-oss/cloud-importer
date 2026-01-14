package azure

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	rhelAIOffer     = "rhelai"
	rhelAIPublisher = "aipcc-cicd"
	rhelAISKU       = "rhelai"
	// Resource Group holding the actual image
	rgName = "aipcc-productization"
)

type rhelaiEphemeralRequest struct {
	vhdPath   string
	imageName string
}

func (a *azureProvider) RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc {
	r := rhelaiEphemeralRequest{
		imageFilePath,
		imageName}
	return r.rhelaiEphemeralRunFunc
}

// This func should add all outputs
func (r rhelaiEphemeralRequest) rhelaiEphemeralRunFunc(ctx *pulumi.Context) error {
	ctx.Export(outOffer, pulumi.String(rhelAIOffer))
	ctx.Export(outPublisher, pulumi.String(rhelAIPublisher))
	ctx.Export(outSKU, pulumi.String(rhelAISKU))
	ctx.Export(outRgName, pulumi.String(rgName))
	ctx.Export(outName, pulumi.String(r.imageName))
	ctx.Export(outArch, pulumi.String("x86_64"))
	location, err := sourceHostingPlace()
	if err != nil {
		return err
	}
	container, sa, rg, err := storageAccount(ctx, location, &r.imageName)
	if err != nil {
		return err
	}
	ctx.Export(outServiceAccountId, sa.ID())
	blobName := "disk.vhd"
	sas := storageAccSAS(ctx, sa.Name, rg.Name)
	sasURL := pulumi.Sprintf(sasURLBase, sa.Name, container.Name, blobName, sas.AccountSasToken())
	blobURI := pulumi.Sprintf(blobURLBase, sa.Name, container.Name, blobName)
	ctx.Export(outBlobURI, blobURI)
	return uploadVHD(ctx, r.vhdPath, sasURL, pulumi.DependsOn([]pulumi.Resource{container}))
}
