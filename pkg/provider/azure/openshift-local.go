package azure

import (
	"fmt"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/bundle"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type openshiftRequest struct {
	bundleURL string
	shasumURL string
	arch      string
}

func (r *openshiftRequest) runFunc(ctx *pulumi.Context) error {
	extractCmd, err := bundle.Extract(ctx, r.bundleURL, r.shasumURL, "azure")
	if err != nil {
		return err
	}

	bundleName, err := bundle.GetBundleNameFromURI(r.bundleURL)
	if err != nil {
		return err
	}

	info, err := bundle.GetBundleInfoFromName(bundleName)
	if err != nil {
		return err
	}

	imageBaseName, err := bundle.GetDescription(r.bundleURL, nil)
	if err != nil {
		return err
	}
	imageName := fmt.Sprintf("%s-%s", *imageBaseName, r.arch)

	container, storageAcc, rg, err := CreateEphemeralStorageAccount(ctx)
	if err != nil {
		return err
	}

	sas := GetStorageAccSAS(ctx, storageAcc.Name, rg.Name)

	sasURL := pulumi.Sprintf(sasURLBase, storageAcc.Name, container.Name, blobName, sas.AccountSasToken())
	blobURL := pulumi.Sprintf(blobURLBase, storageAcc.Name, container.Name, blobName)

	cmd, err := UploadVHD(ctx, bundle.ExtractedVHDDiskFileName, sasURL, pulumi.DependsOn([]pulumi.Resource{rg, storageAcc, container, extractCmd}))
	if err != nil {
		return err
	}

	req := vhdRequest{
		imageName:     imageName,
		arch:          r.arch,
		imageType:     imageTypeSNC,
		version:       info.Version,
		galleryName:   sncGalleryName,
		resourceGroup: sncGalleryRGName,
	}

	return RegisterImage(ctx, storageAcc, req, blobURL, pulumi.DependsOn([]pulumi.Resource{rg, storageAcc, container, cmd}))
}
