package azure

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	rhelAIOffer     = "rhelai"
	rhelAIPublisher = "aipcc-cicd"
	rhelAISKU       = "rhelai"
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
	container, storageAcc, rg, err := storageAccount(ctx)
	if err != nil {
		return err
	}
	ctx.Export(outServiceAccountId, storageAcc.ID())
	blobName := blobName()
	sas := storageAccSAS(ctx, storageAcc.Name, rg.Name)
	sasURL := pulumi.Sprintf(sasURLBase, storageAcc.Name, container.Name, blobName, sas.AccountSasToken())
	blobURI := pulumi.Sprintf(blobURLBase, storageAcc.Name, container.Name, blobName)
	ctx.Export(outBlobURI, blobURI)
	_, err = uploadVHD(ctx, r.vhdPath, sasURL, pulumi.DependsOn([]pulumi.Resource{rg, storageAcc, container}))
	return err

}
