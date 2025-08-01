package azure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type azureProvider struct{}

func Provider() *azureProvider {
	return &azureProvider{}
}

func (p *azureProvider) RHELAI(rawImageFilePath, amiName string) (pulumi.RunFunc, error) {
	// Not implemented for Azure
	return nil, nil
}

func (p *azureProvider) Share(imageID string, targetAccountID string) (pulumi.RunFunc, error) {
	// Not implemented for Azure
	return nil, nil
}

func (p *azureProvider) OpenshiftLocal(bundleURL, shasumURL, arch string) (pulumi.RunFunc, error) {
	// Not implemented for Azure
	return nil, nil
}

func (p *azureProvider) RHELAIOnAzure(subscriptionID, resourceGroup, location, diskPath string, imageName string, tags map[string]string) (pulumi.RunFunc, error) {
	return func(ctx *pulumi.Context) error {

		importer, err := NewAzureImageImporter(subscriptionID, resourceGroup, location, diskPath, imageName, tags)
		if err != nil {
			logging.Fatalf("Failed to create importer: %v", err)
		}

		imageID, err := importer.ImportImage(ctx.Context())
		if err != nil {
			logging.Fatalf("Failed to import image: %v", err)
		}

		logging.Infof("Successfully imported image with ID: %s", imageID)

		ctx.Export("imageName", pulumi.String(imageName))
		return nil
	}, nil
}

type azureImageImporter struct {
	SubscriptionID string
	ResourceGroup  string
	Location       string
	DiskPath       string
	DiskName       string
	ImageName      string
	Tags           map[string]*string
	SizeBytes      int64
	disksClient    *armcompute.DisksClient
	imagesClient   *armcompute.ImagesClient
}

func NewAzureImageImporter(subscriptionID, resourceGroup, location, diskPath string, imageName string, tags map[string]string) (*azureImageImporter, error) {
	fileInfo, err := os.Stat(diskPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk size: %w", err)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create default azure credential: %w", err)
	}

	disksClient, err := armcompute.NewDisksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create disks client: %w", err)
	}

	imagesClient, err := armcompute.NewImagesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create images client: %w", err)
	}

	// Convert map[string]string to map[string]*string
	azureTags := make(map[string]*string)
	for k, v := range tags {
		azureTags[k] = to.Ptr(v)
	}

	return &azureImageImporter{
		SubscriptionID: subscriptionID,
		ResourceGroup:  resourceGroup,
		Location:       location,
		DiskPath:       diskPath,
		DiskName:       fmt.Sprintf("disk-%s-%s", imageName, location),
		ImageName:      imageName,
		Tags:           azureTags,
		SizeBytes:      fileInfo.Size(),
		disksClient:    disksClient,
		imagesClient:   imagesClient,
	}, nil
}

func isNotFound(err error) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}

func (i *azureImageImporter) DiskExists(ctx context.Context) (bool, error) {
	_, err := i.disksClient.Get(ctx, i.ResourceGroup, i.DiskName, nil)
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to get disk: %w", err)
}

func (i *azureImageImporter) ImageExists(ctx context.Context) (bool, error) {
	_, err := i.imagesClient.Get(ctx, i.ResourceGroup, i.ImageName, nil)
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to get image: %w", err)
}

func (i *azureImageImporter) CreateDiskForUpload(ctx context.Context, osType armcompute.OperatingSystemTypes, sku armcompute.DiskStorageAccountTypes) (*armcompute.Disk, error) {
	logging.Infof("Creating disk '%s' for upload...", i.DiskName)
	poller, err := i.disksClient.BeginCreateOrUpdate(ctx, i.ResourceGroup, i.DiskName, armcompute.Disk{
		Location: to.Ptr(i.Location),
		Tags:     i.Tags,
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:    to.Ptr(armcompute.DiskCreateOptionUpload),
				UploadSizeBytes: to.Ptr(i.SizeBytes),
			},
			OSType: to.Ptr(osType),
		},
		SKU: &armcompute.DiskSKU{
			Name: to.Ptr(sku),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin creating disk: %w", err)
	}

	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to finish creating disk: %w", err)
	}
	logging.Infof("Disk '%s' created successfully.", *res.Name)
	return &res.Disk, nil
}

func (i *azureImageImporter) GrantUploadAccess(ctx context.Context, duration int32) (string, error) {
	logging.Infof("Granting SAS write access to disk '%s'...", i.DiskName)
	accessData := armcompute.GrantAccessData{
		Access:            to.Ptr(armcompute.AccessLevelWrite),
		DurationInSeconds: to.Ptr(duration),
	}
	poller, err := i.disksClient.BeginGrantAccess(ctx, i.ResourceGroup, i.DiskName, accessData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin granting access: %w", err)
	}

	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to finish granting access: %w", err)
	}
	return *res.AccessSAS, nil
}

// Note: This function requires 'azcopy' to be installed and in the system's PATH.
func (i *azureImageImporter) UploadVHDToDisk(sasURL string) error {
	logging.Info("Uploading VHD to managed disk via azcopy...")
	cmd := exec.Command("azcopy", "copy", i.DiskPath, sasURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("azcopy command failed: %w", err)
	}
	logging.Info("Upload complete.")
	return nil
}

func (i *azureImageImporter) RevokeUploadAccess(ctx context.Context) error {
	logging.Infof("Revoking SAS access to disk '%s'...", i.DiskName)
	poller, err := i.disksClient.BeginRevokeAccess(ctx, i.ResourceGroup, i.DiskName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin revoking access: %w", err)
	}
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to finish revoking access: %w", err)
	}
	logging.Info("Access revoked.")
	return nil
}

func (i *azureImageImporter) CreateImageFromDisk(ctx context.Context, diskID, hyperVGen string, osType armcompute.OperatingSystemTypes) (*string, error) {
	logging.Infof("Creating managed image '%s' from disk '%s'...", i.ImageName, diskID)
	imageConfig := armcompute.Image{
		Location: to.Ptr(i.Location),
		Tags:     i.Tags,
		Properties: &armcompute.ImageProperties{
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType:      to.Ptr(osType),
					OSState:     to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
					ManagedDisk: &armcompute.SubResource{ID: to.Ptr(diskID)},
					Caching:     to.Ptr(armcompute.CachingTypesReadWrite),
				},
			},
			HyperVGeneration: to.Ptr(armcompute.HyperVGenerationTypes(hyperVGen)),
		},
	}
	poller, err := i.imagesClient.BeginCreateOrUpdate(ctx, i.ResourceGroup, i.ImageName, imageConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin creating image: %w", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to finish creating image: %w", err)
	}
	logging.Infof("Managed image created: %s", *res.ID)
	return res.ID, nil
}

func (i *azureImageImporter) ImportImage(ctx context.Context) (string, error) {
	imageExists, err := i.ImageExists(ctx)
	if err != nil {
		return "", err
	}
	if imageExists {
		logging.Infof("Image '%s' already exists. Skipping.", i.ImageName)
		res, err := i.imagesClient.Get(ctx, i.ResourceGroup, i.ImageName, nil)
		if err != nil {
			return "", fmt.Errorf("failed to get existing image: %w", err)
		}
		return *res.ID, nil
	}

	diskExists, err := i.DiskExists(ctx)
	if err != nil {
		return "", err
	}
	var disk *armcompute.Disk
	if diskExists {
		logging.Infof("Disk '%s' already exists.", i.DiskName)
		res, err := i.disksClient.Get(ctx, i.ResourceGroup, i.DiskName, nil)
		if err != nil {
			return "", fmt.Errorf("failed to get existing disk: %w", err)
		}
		disk = &res.Disk
	} else {
		createdDisk, err := i.CreateDiskForUpload(ctx, armcompute.OperatingSystemTypesLinux, armcompute.DiskStorageAccountTypesStandardLRS)
		if err != nil {
			return "", err
		}
		disk = createdDisk
	}

	sasURL, err := i.GrantUploadAccess(ctx, 86400)
	if err != nil {
		return "", err
	}

	if err := i.UploadVHDToDisk(sasURL); err != nil {
		// Attempt to revoke access even if upload fails, but return the upload error.
		if revokeErr := i.RevokeUploadAccess(ctx); revokeErr != nil {
			logging.Errorf("ERROR: failed to revoke SAS access after upload failure: %v", revokeErr)
		}
		return "", fmt.Errorf("failed to upload VHD to disk: %w", err)
	}

	if err := i.RevokeUploadAccess(ctx); err != nil {
		return "", fmt.Errorf("failed to revoke SAS access: %w", err)
	}

	imageID, err := i.CreateImageFromDisk(ctx, *disk.ID, string(armcompute.HyperVGenerationTypesV2), armcompute.OperatingSystemTypesLinux)
	if err != nil {
		return "", err
	}

	return *imageID, nil
}
