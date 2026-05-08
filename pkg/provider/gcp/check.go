package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
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
		creds, err := google.CredentialsFromJSON(ctx, []byte(credJSON), compute.CloudPlatformScope)
		if err != nil {
			return false, "", fmt.Errorf("failed to parse GOOGLE_CREDENTIALS: %w", err)
		}
		opts = append(opts, option.WithCredentials(creds))
	}

	svc, err := compute.NewService(ctx, opts...)
	if err != nil {
		return false, "", fmt.Errorf("failed to create Compute client: %w", err)
	}

	searchName := sanitizeImageName(imageName)

	img, err := svc.Images.Get(project, searchName).Do()
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "notFound") {
			return false, "", nil
		}
		return false, "", fmt.Errorf("error looking up image %q: %w", imageName, err)
	}
	return true, img.SelfLink, nil
}
