package azure

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
	compute "github.com/pulumi/pulumi-azure-native-sdk/compute/v3"
	resources "github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	storage "github.com/pulumi/pulumi-azure-native-sdk/storage/v3"
	"github.com/pulumi/pulumi-command/sdk/v3/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	sasURLBase  = "https://%s.blob.core.windows.net/%s/%s?%s"
	blobURLBase = "https://%s.blob.core.windows.net/%s/%s"
	// RHELAI
	rhelAIOffer         = "rhelai"
	rhelAIPublisher     = "rhqe"
	rhelAISKU           = "rhqe_rhelai"
	rhelAIGalleryName   = "rhqe_rhelai_images"
	rhelAIGalleryRGName = "cloud-importer-rhelai-image-rg"
	// SNC
	sncOffer         = "snc"
	sncPublisher     = "openshift-local"
	sncSKU           = "openshift_local_snc"
	sncGalleryName   = "crc_openshift_local_snc_images"
	sncGalleryRGName = "cloud-importer-crc-snc-image-rg"

	imageTypeRhelAI = imageType("rhelai")
	imageTypeSNC    = imageType("snc")
)

var (
	blobName = getRandomBlobName()
)

func randomID(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b) // nolint
	id := fmt.Sprintf("cloud-importer-%s-%x", prefix, b)
	return id
}

func getRandomAccountName() string {
	b := make([]byte, 4)
	rand.Read(b) // nolint
	id := fmt.Sprintf("cloudimportersa%x", b)
	return id
}

func getRandomBlobName() string {
	b := make([]byte, 4)
	rand.Read(b) // nolint
	id := fmt.Sprintf("cloud-importer-blobVhd%x.vhd", b)
	return id
}

func CreateEphemeralStorageAccount(ctx *pulumi.Context) (*storage.BlobContainer, *storage.StorageAccount, *resources.ResourceGroup, error) {
	resourceGroup, err := resources.NewResourceGroup(ctx, randomID("resourceGroupSA"), &resources.ResourceGroupArgs{
		ResourceGroupName: pulumi.String(randomID("resourceGroup")),
	}, pulumi.RetainOnDelete(true))
	if err != nil {
		return nil, nil, nil, err
	}

	storageAcc, err := storage.NewStorageAccount(ctx, "storageAccount", &storage.StorageAccountArgs{
		AccountName:       pulumi.String(getRandomAccountName()),
		AccessTier:        storage.AccessTierHot,
		Kind:              pulumi.String(storage.KindStorageV2),
		ResourceGroupName: resourceGroup.Name,
		Sku: &storage.SkuArgs{
			Name: pulumi.String(storage.SkuName_Premium_LRS),
		},
	}, pulumi.DependsOn([]pulumi.Resource{resourceGroup}))
	if err != nil {
		return nil, nil, nil, err
	}

	blobContainer, err := storage.NewBlobContainer(ctx, "blobContainer", &storage.BlobContainerArgs{
		AccountName:       storageAcc.Name,
		ContainerName:     pulumi.String(randomID("blobbox")),
		ResourceGroupName: resourceGroup.Name,
	}, pulumi.DependsOn([]pulumi.Resource{resourceGroup, storageAcc}))
	if err != nil {
		return nil, nil, nil, err
	}
	return blobContainer, storageAcc, resourceGroup, nil
}

func GetStorageAccSAS(ctx *pulumi.Context, storageAccName, rgName pulumi.StringInput) storage.ListStorageAccountSASResultOutput {
	sasPermissions := pulumi.String(storage.PermissionsW + storage.PermissionsR)
	sasServices := pulumi.String(storage.ServicesB)
	sasResourceTypes := pulumi.String(storage.SignedResourceTypesO)
	sasExpiry := pulumi.String(time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339Nano))
	sasProtocol := storage.HttpProtocolHttps

	sas := storage.ListStorageAccountSASOutput(ctx, storage.ListStorageAccountSASOutputArgs{
		AccountName:            storageAccName,
		ResourceGroupName:      rgName,
		Permissions:            sasPermissions,
		Services:               sasServices,
		ResourceTypes:          sasResourceTypes,
		SharedAccessExpiryTime: sasExpiry,
		Protocols:              &sasProtocol,
	})
	return sas
}

func UploadVHD(ctx *pulumi.Context, vhdPath string, blobSASUrl pulumi.StringInput, opts pulumi.ResourceOrInvokeOption) (*local.Command, error) {
	azcopyCmd, err := local.NewCommand(ctx, "azcopy-vhd", &local.CommandArgs{
		Create: pulumi.Sprintf("azcopy cp %s '%s' --blob-type='PageBlob'", vhdPath, blobSASUrl),
	}, opts)
	if err != nil {
		return nil, err
	}
	return azcopyCmd, nil
}

func CreateGallery(ctx *pulumi.Context, rg *resources.ResourceGroup, req vhdRequest, opts pulumi.ResourceOrInvokeOption) (*compute.Gallery, error) {
	// check if a gallery already exists
	_, err := compute.LookupGallery(ctx, &compute.LookupGalleryArgs{
		GalleryName:       req.galleryName,
		ResourceGroupName: req.resourceGroup,
	}, opts)
	if err == nil {
		return nil, err
	}
	gallery, err := compute.NewGallery(ctx, "ComputeGallery", &compute.GalleryArgs{
		Description:       pulumi.String(req.imageDesc),
		GalleryName:       pulumi.String(req.galleryName),
		Location:          rg.Location,
		ResourceGroupName: rg.Name,
	}, opts, pulumi.RetainOnDelete(true))
	if err != nil {
		return nil, err
	}

	return gallery, nil
}

func CreateGalleryImageDefinition(ctx *pulumi.Context, rg *resources.ResourceGroup, req vhdRequest, opts pulumi.ResourceOrInvokeOption) (*compute.GalleryImage, error) {
	_, err := compute.LookupGalleryImage(ctx, &compute.LookupGalleryImageArgs{
		GalleryImageName:  req.imageName,
		GalleryName:       req.galleryName,
		ResourceGroupName: req.resourceGroup,
	}, opts)
	if err == nil {
		return nil, nil
	}

	galleryImageArgs := &compute.GalleryImageArgs{
		GalleryImageName:  pulumi.String(req.imageName),
		Description:       pulumi.String(req.imageDesc),
		GalleryName:       pulumi.String(req.galleryName),
		ResourceGroupName: rg.Name,
		Location:          rg.Location,
		Architecture: func() pulumi.StringPtrInput {
			if req.arch == "x86_64" {
				return compute.ArchitectureX64
			}
			return compute.ArchitectureArm64
		}(),
		HyperVGeneration: compute.HyperVGenerationTypesV2,
		OsType:           compute.OperatingSystemTypesLinux,
		OsState:          compute.OperatingSystemStateTypesGeneralized,
	}

	switch req.imageType {
	case imageTypeRhelAI:
		galleryImageArgs.Identifier = compute.GalleryImageIdentifierArgs{
			Offer:     pulumi.String(rhelAIOffer),
			Publisher: pulumi.String(rhelAIPublisher),
			Sku:       pulumi.String(rhelAISKU),
		}
	case imageTypeSNC:
		galleryImageArgs.Identifier = compute.GalleryImageIdentifierArgs{
			Offer:     pulumi.String(sncOffer),
			Publisher: pulumi.String(sncPublisher),
			Sku:       pulumi.String(sncSKU),
		}
	default:
		return nil, errors.New("image type not supported")
	}

	imageDefinition, err := compute.NewGalleryImage(ctx, "GalleryImageDef", galleryImageArgs, opts, pulumi.RetainOnDelete(true))
	if err != nil {
		return nil, err
	}
	return imageDefinition, err
}

func CreateGalleryImageVersion(ctx *pulumi.Context, rg *resources.ResourceGroup, sa *storage.StorageAccount, req vhdRequest, blobURL pulumi.StringInput, opts pulumi.ResourceOrInvokeOption) (*compute.GalleryImageVersion, error) {
	_, err := compute.LookupGalleryImageVersion(ctx, &compute.LookupGalleryImageVersionArgs{
		GalleryImageVersionName: req.version,
		GalleryImageName:        req.imageName,
		GalleryName:             req.galleryName,
		ResourceGroupName:       req.resourceGroup,
	}, opts)
	if err == nil {
		return nil, nil
	}

	galleryImageVersion, err := compute.NewGalleryImageVersion(ctx, "GalleryImageVer", &compute.GalleryImageVersionArgs{
		GalleryName:             pulumi.String(req.galleryName),
		GalleryImageName:        pulumi.String(req.imageName),
		GalleryImageVersionName: pulumi.String(req.version),
		ResourceGroupName:       rg.Name,
		Location:                rg.Location,
		PublishingProfile: compute.GalleryImageVersionPublishingProfileArgs{
			StorageAccountType: compute.StorageAccountType_Premium_LRS,
			ReplicationMode:    compute.ReplicationModeFull,
			TargetRegions:      genTargetRegionsArray(req.regions),
		},
		StorageProfile: compute.GalleryImageVersionStorageProfileArgs{
			OsDiskImage: &compute.GalleryOSDiskImageArgs{
				Source: &compute.GalleryDiskImageSourceArgs{
					StorageAccountId: sa.ID(),
					Uri:              blobURL,
				},
			},
		},
	}, opts, pulumi.RetainOnDelete(true))
	if err != nil {
		return nil, err
	}
	return galleryImageVersion, err
}

func CheckGalleryImageExists(ctx *pulumi.Context, req vhdRequest) bool {
	_, err := compute.LookupGalleryImageVersion(ctx, &compute.LookupGalleryImageVersionArgs{
		GalleryImageVersionName: req.version,
		GalleryImageName:        req.imageName,
		GalleryName:             req.galleryName,
		ResourceGroupName:       req.resourceGroup,
	})
	if err != nil {
		logging.Debugf("Unable to find the image: %s: %v",
			fmt.Sprintf("%s/%s/%s", req.galleryName, req.imageName, req.version), err)
		return false
	}
	return true
}

func ReplicateGalleryImageVersion(ctx *pulumi.Context, req vhdRequest) error {
	imgVer, err := compute.LookupGalleryImageVersion(ctx, &compute.LookupGalleryImageVersionArgs{
		GalleryImageVersionName: req.version,
		GalleryImageName:        req.imageName,
		GalleryName:             req.galleryName,
		ResourceGroupName:       req.resourceGroup,
	})
	if err != nil {
		return fmt.Errorf("Unable to find the image: %s: %w",
			fmt.Sprintf("%s/%s/%s", req.galleryName, req.imageName, req.version), err)
	}

	var rg *resources.ResourceGroup
	r, err := resources.LookupResourceGroup(ctx, &resources.LookupResourceGroupArgs{
		ResourceGroupName: req.resourceGroup,
	})
	if err == nil {
		rg, err = resources.GetResourceGroup(ctx, req.resourceGroup, pulumi.ID(r.Id), nil)
		if err != nil {
			return err
		}
	}

	var regions = req.regions
	for _, reg := range imgVer.PublishingProfile.TargetRegions {
		regions = append(regions, reg.Name)
	}

	_, err = compute.NewGalleryImageVersion(ctx, "GalleryImageVer", &compute.GalleryImageVersionArgs{
		GalleryName:             pulumi.String(req.galleryName),
		GalleryImageName:        pulumi.String(req.imageName),
		GalleryImageVersionName: pulumi.String(req.version),
		ResourceGroupName:       rg.Name,
		Location:                rg.Location,
		PublishingProfile: compute.GalleryImageVersionPublishingProfileArgs{
			StorageAccountType: compute.StorageAccountType_Premium_LRS,
			ReplicationMode:    compute.ReplicationModeFull,
			TargetRegions:      genTargetRegionsArray(regions),
		},
		StorageProfile: compute.GalleryImageVersionStorageProfileArgs{
			OsDiskImage: &compute.GalleryOSDiskImageArgs{
				Source: &compute.GalleryDiskImageSourceArgs{
					StorageAccountId: pulumi.String(*imgVer.StorageProfile.OsDiskImage.Source.StorageAccountId),
					Uri:              pulumi.String(*imgVer.StorageProfile.OsDiskImage.Source.Uri),
				},
			},
		},
	}, pulumi.RetainOnDelete(true), pulumi.Import(pulumi.ID(imgVer.Id)))
	if err != nil {
		return err
	}
	return nil
}

func genTargetRegionsArray(regions []string) compute.TargetRegionArray {
	var targetRegions = compute.TargetRegionArray{}
	var availableRegions = regions

	for _, reg := range availableRegions {
		targetRegions = append(targetRegions, &compute.TargetRegionArgs{
			Name:                 pulumi.String(reg),
			RegionalReplicaCount: pulumi.Int(1),
			ExcludeFromLatest:    pulumi.Bool(false),
		})
	}
	return targetRegions
}

type imageType string

type vhdRequest struct {
	imageName     string
	imageDesc     string
	galleryName   string
	version       string
	arch          string
	imageType     imageType
	resourceGroup string
	regions       []string
}

func RegisterImage(ctx *pulumi.Context, sa *storage.StorageAccount, req vhdRequest, blobURL pulumi.StringInput, opts pulumi.ResourceOrInvokeOption) error {
	var rg *resources.ResourceGroup
	var err error

	// check if resource group already exists
	r, err := resources.LookupResourceGroup(ctx, &resources.LookupResourceGroupArgs{
		ResourceGroupName: req.resourceGroup,
	}, opts)
	if err == nil {
		rg, err = resources.GetResourceGroup(ctx, req.resourceGroup, pulumi.ID(r.Id), nil)
		if err != nil {
			return err
		}
	} else {
		// create the resource group using provided name
		rg, err = resources.NewResourceGroup(ctx, randomID("resourceGroupGallery"), &resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(req.resourceGroup),
		}, pulumi.RetainOnDelete(true))
		if err != nil {
			return err
		}
	}

	gallery, err := CreateGallery(ctx, rg, req, opts)
	if err != nil {
		return err
	}

	def, err := CreateGalleryImageDefinition(ctx, rg, req, pulumi.DependsOn([]pulumi.Resource{gallery}))
	if err != nil {
		return err
	}

	_, err = CreateGalleryImageVersion(ctx, rg, sa, req, blobURL, pulumi.DependsOn([]pulumi.Resource{def, sa}))
	if err != nil {
		return err
	}
	return nil
}
