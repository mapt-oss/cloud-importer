package azure

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
)

const (
	envAzureStorageAccount = "AZURE_STORAGE_ACCOUNT"
	pulumiLocksPath        = ".pulumi/locks"
)

func newBlobClient() (*azblob.Client, error) {
	storageAccount := os.Getenv(envAzureStorageAccount)
	if len(storageAccount) == 0 {
		return nil, fmt.Errorf("%s environment variable is not set", envAzureStorageAccount)
	}
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net", storageAccount)

	if key := os.Getenv("AZURE_STORAGE_KEY"); key != "" {
		cred, err := azblob.NewSharedKeyCredential(storageAccount, key)
		if err != nil {
			return nil, fmt.Errorf("failed to create shared key credential: %w", err)
		}
		return azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	}

	if sasToken := os.Getenv("AZURE_STORAGE_SAS_TOKEN"); sasToken != "" {
		return azblob.NewClientWithNoCredential(serviceURL+"?"+sasToken, nil)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}
	return azblob.NewClient(serviceURL, cred, nil)
}

func parseAzblobBackedURL(backedURL string) (container string, path string, err error) {
	if !strings.HasPrefix(backedURL, "azblob://") {
		return "", "", fmt.Errorf("invalid azblob URI: must start with azblob://")
	}
	u, err := url.Parse(backedURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse azblob URI: %w", err)
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}

func deleteBlobs(containerName, prefix string) error {
	client, err := newBlobClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	blobNames, err := listBlobNames(ctx, client, containerName, prefix)
	if err != nil {
		return err
	}
	for _, name := range blobNames {
		if _, err := client.DeleteBlob(ctx, containerName, name, nil); err != nil {
			logging.Warnf("Failed to delete blob %s: %v", name, err)
		}
	}
	return nil
}

func listBlobNames(ctx context.Context, client *azblob.Client, containerName, prefix string) ([]string, error) {
	var names []string
	pager := client.NewListBlobsFlatPager(containerName, &azblob.ListBlobsFlatOptions{
		Prefix: &prefix,
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing blobs: %w", err)
		}
		for _, blob := range page.Segment.BlobItems {
			names = append(names, *blob.Name)
		}
	}
	return names, nil
}

func DeleteLocks(backedURL string) {
	logging.Infof("Force destroy: removing Pulumi locks from %s", backedURL)
	container, path, err := parseAzblobBackedURL(backedURL)
	if err != nil {
		logging.Warnf("Force destroy: failed to parse Azure Blob backend URL: %v", err)
		return
	}
	prefix := pulumiLocksPath + "/"
	if path != "" {
		prefix = path + "/" + prefix
	}
	if err := deleteBlobs(container, prefix); err != nil {
		logging.Warnf("Force destroy: failed to remove locks from Azure Blob: %v", err)
		return
	}
	logging.Info("Force destroy: successfully removed Pulumi locks from Azure Blob Storage")
}

func CleanupState(backedURL string) {
	logging.Infof("Cleaning up Pulumi state from %s", backedURL)
	container, path, err := parseAzblobBackedURL(backedURL)
	if err != nil {
		logging.Warnf("Failed to parse Azure Blob backend URL: %v", err)
		return
	}
	prefix := ".pulumi/"
	if path != "" {
		prefix = path + "/"
	}
	if err := deleteBlobs(container, prefix); err != nil {
		logging.Warnf("Failed to cleanup Azure Blob state: %v", err)
		return
	}
	logging.Info("Successfully cleaned up Pulumi state from Azure Blob Storage")
}
