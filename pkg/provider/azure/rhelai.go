package azure

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type rhelAIRequest struct {
	vhdPath   string
	imageName string
}

func (r *rhelAIRequest) runFunc(ctx *pulumi.Context) error {
	container, storageAcc, rg, err := CreateEphemeralStorageAccount(ctx)
	if err != nil {
		return err
	}

	sas := GetStorageAccSAS(ctx, storageAcc.Name, rg.Name)

	sasURL := pulumi.Sprintf(sasURLBase, storageAcc.Name, container.Name, blobName, sas.AccountSasToken())
	blobURL := pulumi.Sprintf(blobURLBase, storageAcc.Name, container.Name, blobName)

	cmd, err := UploadVHD(ctx, r.vhdPath, sasURL, pulumi.DependsOn([]pulumi.Resource{rg, storageAcc, container}))
	if err != nil {
		return err
	}
	vhdRequest := vhdRequest{
		imageName:     r.imageName,
		arch:          "x86_64",
		imageType:     imageTypeRhelAI,
		version:       "10",
		galleryName:   rhelAIGalleryName,
		resourceGroup: rhelAIGalleryRGName,
	}

	return RegisterImage(ctx, storageAcc, vhdRequest, blobURL, pulumi.DependsOn([]pulumi.Resource{rg, storageAcc, container, cmd}))
}
