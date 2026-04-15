package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	checkRGName = rhelAIRGName
)

func (a *azureProvider) ImageExists(imageName string) (bool, string, error) {
	galleryName := strings.ReplaceAll(imageName, "-", "_")

	cred, subscriptionID, err := getCredentials()
	if err != nil {
		return false, "", fmt.Errorf("error getting Azure credentials: %w", err)
	}

	client, err := armcompute.NewGalleryImagesClient(*subscriptionID, cred, nil)
	if err != nil {
		return false, "", fmt.Errorf("error creating gallery images client: %w", err)
	}

	resp, err := client.Get(context.Background(), checkRGName, galleryName, imageName, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
			return false, "", nil
		}
		return false, "", fmt.Errorf("error checking image: %w", err)
	}
	return true, *resp.GalleryImage.ID, nil
}
