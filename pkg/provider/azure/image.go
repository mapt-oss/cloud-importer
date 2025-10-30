package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/pulumi/pulumi-azure-native-sdk/compute/v3"
	resources "github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var (
	// ImageRegister should get this values from the ephemeralResults
	outName             = "name"
	outArch             = "arch"
	outOffer            = "offer"
	outPublisher        = "publisher"
	outSKU              = "sku"
	outServiceAccountId = "saId"
	outBlobURI          = "blobURI"
)

func (a *azureProvider) ImageRegister(ephemeralResults auto.UpResult, replicate bool, orgId string) (pulumi.RunFunc, error) {
	name, ok := ephemeralResults.Outputs[outName]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outName)
	}
	arch, ok := ephemeralResults.Outputs[outArch]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outArch)
	}
	offer, ok := ephemeralResults.Outputs[outOffer]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outOffer)
	}
	publisher, ok := ephemeralResults.Outputs[outPublisher]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outPublisher)
	}
	sku, ok := ephemeralResults.Outputs[outSKU]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outSKU)
	}
	saId, ok := ephemeralResults.Outputs[outServiceAccountId]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outServiceAccountId)
	}
	blobURI, ok := ephemeralResults.Outputs[outBlobURI]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outBlobURI)
	}
	r := regiterRequest{
		name:             name.Value.(string),
		arch:             arch.Value.(string),
		offer:            offer.Value.(string),
		publisher:        publisher.Value.(string),
		sku:              sku.Value.(string),
		storageAccountId: saId.Value.(string),
		blobURI:          blobURI.Value.(string),
		replicate:        replicate,
		orgTenantId:      &orgId,
	}
	return r.registerFunc, nil
}

type regiterRequest struct {
	name                  string
	arch                  string
	offer, publisher, sku string
	storageAccountId      string
	blobURI               string
	replicate             bool
	orgTenantId           *string
}

// from an image as a raw on a s3 bucket this function will import it as a snapshot
// and the register the snapshot as an AMI
func (r *regiterRequest) registerFunc(ctx *pulumi.Context) error {
	rg, err := resources.NewResourceGroup(
		ctx,
		"rg",
		&resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(randomize(r.name)),
		})
	if err != nil {
		return err
	}
	gArgs := &compute.GalleryArgs{
		Description:       pulumi.String(r.name),
		GalleryName:       pulumi.String(r.name),
		Location:          rg.Location,
		ResourceGroupName: rg.Name,
	}
	if r.orgTenantId != nil {
		gArgs.SharingProfile = &compute.SharingProfileArgs{
			Permissions: compute.GallerySharingPermissionTypesGroups}
	}
	g, err := compute.NewGallery(ctx,
		"gallery",
		gArgs)
	if err != nil {
		return err
	}
	gi, err := compute.NewGalleryImage(ctx,
		"image",
		&compute.GalleryImageArgs{
			GalleryImageName:  pulumi.String(r.name),
			Description:       pulumi.String(r.name),
			GalleryName:       g.Name,
			ResourceGroupName: rg.Name,
			Location:          rg.Location,
			Architecture: func() pulumi.StringPtrInput {
				if r.arch == "x86_64" {
					return compute.ArchitectureX64
				}
				return compute.ArchitectureArm64
			}(),
			HyperVGeneration: compute.HyperVGenerationTypesV2,
			OsType:           compute.OperatingSystemTypesLinux,
			OsState:          compute.OperatingSystemStateTypesGeneralized,
			Identifier: compute.GalleryImageIdentifierArgs{
				Offer:     pulumi.String(r.offer),
				Publisher: pulumi.String(r.publisher),
				Sku:       pulumi.String(r.sku),
			},
		})
	if err != nil {
		return err
	}
	targetRegions, err := targetRegions()
	if err != nil {
		return err
	}
	_, err = compute.NewGalleryImageVersion(ctx,
		"GalleryImageVer",
		&compute.GalleryImageVersionArgs{
			GalleryName:             g.Name,
			GalleryImageName:        gi.Name,
			GalleryImageVersionName: pulumi.String(r.name),
			ResourceGroupName:       rg.Name,
			Location:                rg.Location,
			PublishingProfile: compute.GalleryImageVersionPublishingProfileArgs{
				StorageAccountType: compute.StorageAccountType_Premium_LRS,
				ReplicationMode:    compute.ReplicationModeFull,
				TargetRegions:      targetRegions,
			},
			StorageProfile: compute.GalleryImageVersionStorageProfileArgs{
				OsDiskImage: &compute.GalleryOSDiskImageArgs{
					Source: &compute.GalleryDiskImageSourceArgs{
						StorageAccountId: pulumi.String(r.storageAccountId),
						Uri:              pulumi.String(r.blobURI),
					},
				},
			},
		})
	if err != nil {
		return err
	}
	if r.orgTenantId != nil {
		pulumi.All(rg.Name, g.Name).ApplyT(func(args []interface{}) error {
			rgName := args[0].(string)
			galleryName := args[1].(string)
			// go func() {
			return shareGallery(*r.orgTenantId, rgName, galleryName)
			// 	if err != nil {
			// 		return err
			// 	}
			// // }()
			// return nil
		})
	}
	return err
}

func shareGallery(tenantID, rgName, galleryName string) error {
	cred, subscriptionID, err := getCredentials()
	if err != nil {
		return err
	}
	ctx := context.Background()
	client, err := armcompute.NewGalleriesClient(*subscriptionID, cred, nil)
	if err != nil {
		return err
	}
	p := armcompute.GallerySharingPermissionTypesGroups
	a := armcompute.SharingProfileGroupTypesAADTenants
	poller, err := client.BeginUpdate(
		ctx,
		rgName,
		galleryName,
		armcompute.GalleryUpdate{
			Properties: &armcompute.GalleryProperties{
				SharingProfile: &armcompute.SharingProfile{
					Permissions: &p,
					Groups: []*armcompute.SharingProfileGroup{
						{
							Type: &a,
							IDs:  []*string{&tenantID},
						},
					},
				},
			},
		},
		nil,
	)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
