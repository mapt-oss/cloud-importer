package gcp

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mapt-oss/cloud-importer/pkg/util"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

func (p *gcpProvider) ImageExists(imageName string) (bool, string, error) {
	project := os.Getenv("GOOGLE_PROJECT")
	if project == "" {
		return false, "", fmt.Errorf("GOOGLE_PROJECT is not set")
	}

	ctx := context.Background()

	var opts []option.ClientOption
	if credJSON := os.Getenv("GOOGLE_CREDENTIALS"); credJSON != "" {
		// Secure, non-deprecated way to pass JSON credentials directly to the client
		opts = append(opts, option.WithCredentialsJSON([]byte(credJSON)))
	}

	svc, err := compute.NewService(ctx, opts...)
	if err != nil {
		return false, "", fmt.Errorf("failed to create Compute client: %w", err)
	}

	searchName := util.SanitizeBucketName(imageName)

	img, err := svc.Images.Get(project, searchName).Do()
	if err != nil {
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && gErr.Code == 404 {
			return false, "", nil
		}
		return false, "", fmt.Errorf("error looking up image %q: %w", imageName, err)
	}
	return true, img.SelfLink, nil
}
