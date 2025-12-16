package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	imgctx "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	"github.com/pulumi/pulumi-azure-native-sdk/compute/v3"
	resources "github.com/pulumi/pulumi-azure-native-sdk/resources/v3"

	// az "github.com/pulumi/pulumi-azure-native-sdk/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	galleryScopeFormat = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s"
	// Id corresponds with a built-in role
	vmContributorRoleFormat = "/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c"
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

func (a *azureProvider) ImageRegister(ephemeralResults auto.UpResult, replicate bool, shareOrgIds []string) (pulumi.RunFunc, error) {
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
		shareTenantIds:   shareOrgIds,
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
	shareTenantIds        []string
}

// from an image as a raw on a s3 bucket this function will import it as a snapshot
// and the register the snapshot as an AMI
func (r *regiterRequest) registerFunc(ctx *pulumi.Context) error {
	// prov, err := az.NewProvider(ctx, "prov", &az.ProviderArgs{
	// 	SkipProviderRegistration: pulumi.Bool(true),
	// })
	// if err != nil {
	// 	return err
	// }
	// opts := pulumi.Provider(prov)
	location, err := sourceHostingPlace()
	if err != nil {
		return err
	}
	rg, err := resources.NewResourceGroup(
		ctx,
		"rg",
		&resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(r.name),
			Location:          pulumi.String(*location),
			Tags:              pulumi.ToStringMap(imgctx.GetTagsMap()),
		})
	if err != nil {
		return err
	}
	gName := strings.ReplaceAll(r.name, "-", "_")
	gArgs := &compute.GalleryArgs{
		Description:       pulumi.String(r.name),
		GalleryName:       pulumi.String(gName),
		Location:          rg.Location,
		ResourceGroupName: rg.Name,
		Tags:              pulumi.ToStringMap(imgctx.GetTagsMap()),
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
			Tags: pulumi.ToStringMap(imgctx.GetTagsMap()),
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
			GalleryImageVersionName: pulumi.String("1.0.0"),
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
			Tags: pulumi.ToStringMap(imgctx.GetTagsMap()),
		})
	if err != nil {
		return err
	}
	if len(r.shareTenantIds) > 0 {
		pulumi.All(rg.Name, g.Name).ApplyT(func(args []interface{}) error {
			rgName := args[0].(string)
			galleryName := args[1].(string)
			return shareGallery(r.shareTenantIds, rgName, galleryName, r.name)
		})
	}
	return err
}

func shareGallery(sharTenantIDs []string, rgName, galleryName, imageName string) error {
	cred, subscriptionID, err := getCredentials()
	if err != nil {
		return err
	}
	client, err := armauthorization.NewRoleAssignmentsClient(*subscriptionID, cred, nil)
	if err != nil {
		return err
	}
	vmContributorRoleID := fmt.Sprintf(vmContributorRoleFormat, *subscriptionID)
	for _, tenantId := range sharTenantIDs {
		_, err = client.Create(context.Background(),
			fmt.Sprintf(galleryScopeFormat, *subscriptionID, rgName, galleryName),
			fmt.Sprintf("vm-contributor%s", imageName),
			armauthorization.RoleAssignmentCreateParameters{
				Properties: &armauthorization.RoleAssignmentProperties{
					PrincipalID:      &tenantId,
					RoleDefinitionID: &vmContributorRoleID,
				},
			}, nil)
		return err
	}
	return nil
}
