package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/util"
	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/compute"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (p *gcpProvider) ImageRegister(ephemeralResults auto.UpResult, replicate bool, shareProjectIds []string) (pulumi.RunFunc, func(context.Context), error) {
	if err := validateShareProjectIds(shareProjectIds); err != nil {
		return nil, nil, err
	}
	imageNameOutput, ok := ephemeralResults.Outputs[outImageName]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outImageName)
	}
	gcsURIOutput, ok := ephemeralResults.Outputs[outGCSURI]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outGCSURI)
	}

	r := gcpRegisterRequest{
		imageName:       imageNameOutput.Value.(string),
		gcsURI:          gcsURIOutput.Value.(string),
		replicate:       replicate,
		shareProjectIds: shareProjectIds,
	}
	return r.registerFunc, nil, nil
}

type gcpRegisterRequest struct {
	imageName       string
	gcsURI          string
	replicate       bool
	shareProjectIds []string
}

// replicateMultiRegions are the three GCP multi-regions used when --replicate is set.
// An image copy stored in each eliminates the per-region first-launch transfer penalty
// for zones within that multi-region. Zones outside all three (Australia, Canada,
// South America, Middle East, Africa) fall back to the canonical primary image.
var replicateMultiRegions = []string{"us", "eu", "asia"}

func (r *gcpRegisterRequest) registerFunc(ctx *pulumi.Context) error {
	baseName := util.SanitizeBucketName(r.imageName)

	// GCP only allows a single storageLocation per image. Default to "us" (the
	// US multi-region). Override via GOOGLE_IMAGE_STORAGE_LOCATIONS (single value,
	// e.g. "eu" or "asia"). The image remains globally accessible regardless of
	// which multi-region its data is stored in; this setting only affects where the
	// underlying bytes are cached (which impacts first-boot latency in distant regions).
	storageLocation := os.Getenv("GOOGLE_IMAGE_STORAGE_LOCATIONS")
	if storageLocation == "" {
		storageLocation = "us"
	}

	// Primary image — created from the GCS raw disk upload. Acts as the canonical
	// fallback for zones not covered by any GCP multi-region (Australia, Canada,
	// South America, Middle East, Africa). When --replicate is set, consumer tooling
	// should prefer the regional copies (baseName-us, baseName-eu, baseName-asia)
	// over this primary to get optimal boot times.
	primary, err := compute.NewImage(ctx, "image", &compute.ImageArgs{
		Name:             pulumi.String(baseName),
		Description:      pulumi.String(r.imageName),
		StorageLocations: pulumi.StringArray{pulumi.String(strings.TrimSpace(storageLocation))},
		RawDisk: &compute.ImageRawDiskArgs{
			// Compute Engine API requires https:// URL, not gs:// URI.
			Source: pulumi.String(strings.Replace(r.gcsURI, "gs://", "https://storage.googleapis.com/", 1)),
		},
	}, pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "6h", Update: "6h", Delete: "30m"}))
	if err != nil {
		return err
	}
	if err := r.applyIAM(ctx, primary, ""); err != nil {
		return err
	}

	if !r.replicate {
		return nil
	}

	// --replicate: create per-multi-region copies via SourceImage (no re-upload; GCP
	// copies internally). Consumer tooling maps zone prefix to image name:
	//   us-*     → baseName-us
	//   europe-* → baseName-eu
	//   asia-*   → baseName-asia
	//   others   → baseName (canonical fallback above)
	logging.Infof("--replicate: creating %s-{us,eu,asia} with per-multi-region storageLocations", baseName)
	for _, region := range replicateMultiRegions {
		// Truncate baseName so "baseName-{region}" stays within GCP's 63-char limit.
		maxBase := 63 - 1 - len(region)
		name := baseName
		if len(name) > maxBase {
			name = strings.TrimRight(name[:maxBase], "-")
		}

		copy, err := compute.NewImage(ctx, "image-"+region, &compute.ImageArgs{
			Name:             pulumi.String(name + "-" + region),
			Description:      pulumi.String(r.imageName + " (" + region + ")"),
			StorageLocations: pulumi.StringArray{pulumi.String(region)},
			SourceImage:      primary.SelfLink.ToStringPtrOutput(),
		}, pulumi.DependsOn([]pulumi.Resource{primary}),
			pulumi.Timeouts(&pulumi.CustomTimeouts{Create: "2h", Update: "2h", Delete: "30m"}))
		if err != nil {
			return err
		}
		if err := r.applyIAM(ctx, copy, region); err != nil {
			return err
		}
	}
	return nil
}

// applyIAM grants roles/compute.imageUser on img to each project's Compute Engine
// service agent. suffix is appended to Pulumi resource names to keep them unique
// across the primary and replicated images.
func (r *gcpRegisterRequest) applyIAM(ctx *pulumi.Context, img *compute.Image, suffix string) error {
	resourceSuffix := ""
	if suffix != "" {
		resourceSuffix = "-" + suffix
	}
	for _, projectNumber := range r.shareProjectIds {
		member := fmt.Sprintf("serviceAccount:service-%s@compute-system.iam.gserviceaccount.com", projectNumber)
		_, err := compute.NewImageIamMember(ctx,
			fmt.Sprintf("share-%s%s", projectNumber, resourceSuffix),
			&compute.ImageIamMemberArgs{
				Image:  img.Name,
				Role:   pulumi.String("roles/compute.imageUser"),
				Member: pulumi.String(member),
			})
		if err != nil {
			return err
		}
	}
	return nil
}
