package gcp

import (
	"fmt"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/organizations"
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

	image, err := compute.NewImage(ctx, "image", &compute.ImageArgs{
		Name:        pulumi.String(imageName),
		Description: pulumi.String(r.imageName),
		RawDisk: &compute.ImageRawDiskArgs{
			// Compute Engine API requires https:// URL, not gs:// URI.
			Source: pulumi.String(strings.Replace(r.gcsURI, "gs://", "https://storage.googleapis.com/", 1)),
		},
	})
	if err != nil {
		return err
	}

	for _, projectId := range r.shareProjectIds {
		proj, err := organizations.LookupProject(ctx, &organizations.LookupProjectArgs{
			ProjectId: &projectId,
		}, nil)
		if err != nil {
			return fmt.Errorf("failed to look up GCP project %q: %w", projectId, err)
		}
		member := fmt.Sprintf("serviceAccount:service-%s@compute-system.iam.gserviceaccount.com", proj.Number)
		_, err = compute.NewImageIamBinding(ctx,
			fmt.Sprintf("share-%s", projectId),
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
