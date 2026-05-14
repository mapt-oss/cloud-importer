package gcp

import (
	"fmt"
	"os"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (p *gcpProvider) ImageRegister(ephemeralResults auto.UpResult, replicate bool, shareProjectIds []string) (pulumi.RunFunc, error) {
	imageNameOutput, ok := ephemeralResults.Outputs[outImageName]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outImageName)
	}
	gcsURIOutput, ok := ephemeralResults.Outputs[outGCSURI]
	if !ok {
		return nil, fmt.Errorf("output not found: %s", outGCSURI)
	}

	r := gcpRegisterRequest{
		imageName:       imageNameOutput.Value.(string),
		gcsURI:          gcsURIOutput.Value.(string),
		replicate:       replicate,
		shareProjectIds: shareProjectIds,
	}
	return r.registerFunc, nil
}

type gcpRegisterRequest struct {
	imageName       string
	gcsURI          string
	replicate       bool
	shareProjectIds []string
}

func (r *gcpRegisterRequest) registerFunc(ctx *pulumi.Context) error {
	if r.replicate {
		logging.Info("--replicate has no effect for GCP: Compute Engine images are already globally available within a project")
	}

	imageName := sanitizeImageName(r.imageName)

	// GCP only allows a single storageLocation per image. Default to "us" (the
	// US multi-region). Override via GOOGLE_IMAGE_STORAGE_LOCATIONS (single value,
	// e.g. "eu" or "asia"). The image remains globally accessible regardless of
	// which multi-region its data is stored in; this setting only affects where the
	// underlying bytes are cached (which impacts first-boot latency in distant regions).
	storageLocation := os.Getenv("GOOGLE_IMAGE_STORAGE_LOCATIONS")
	if storageLocation == "" {
		storageLocation = "us"
	}
	storageLocations := pulumi.StringArray{pulumi.String(strings.TrimSpace(storageLocation))}

	image, err := compute.NewImage(ctx, "image", &compute.ImageArgs{
		Name:             pulumi.String(imageName),
		Description:      pulumi.String(r.imageName),
		StorageLocations: storageLocations,
		RawDisk: &compute.ImageRawDiskArgs{
			// Compute Engine API requires https:// URL, not gs:// URI.
			Source: pulumi.String(strings.Replace(r.gcsURI, "gs://", "https://storage.googleapis.com/", 1)),
		},
	}, pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Update: "6h", Delete: "30m"}))
	if err != nil {
		return err
	}

	for _, projectNumber := range r.shareProjectIds {
		// --share-orgs-ids for GCP takes project numbers (e.g. 571214177986), visible
		// in the GCP Console next to the project ID. Using the number directly avoids
		// a Cloud Resource Manager API call and removes that API enablement as a
		// prerequisite. The Compute Engine service agent email format is fixed.
		member := fmt.Sprintf("serviceAccount:service-%s@compute-system.iam.gserviceaccount.com", projectNumber)
		_, err = compute.NewImageIamBinding(ctx,
			fmt.Sprintf("share-%s", projectNumber),
			&compute.ImageIamBindingArgs{
				Image: image.Name,
				Role:  pulumi.String("roles/compute.imageUser"),
				Members: pulumi.StringArray{
					pulumi.String(member),
				},
			})
		if err != nil {
			return err
		}
	}
	return nil
}
