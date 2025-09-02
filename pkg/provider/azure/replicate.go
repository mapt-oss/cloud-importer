package azure

import (
	"strings"

	"github.com/pulumi/pulumi-azure-native-sdk/compute/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type replicateRequest struct {
	galleryImageName string
	targetRegions    []string
}

func (r *replicateRequest) runFunc(ctx *pulumi.Context) error {
	req := vhdRequest{
		imageName:     r.galleryImageName,
		version:       r.getImageVersion(),
		regions:       r.targetRegions,
		imageType:     r.getImageType(),
		galleryName:   r.getGalleryName(),
		arch:          r.getArch(),
		resourceGroup: r.getResourceGroupName(),
	}
	return ReplicateGalleryImageVersion(ctx, req)
}

func (r *replicateRequest) getArch() string {
	if strings.Contains(r.galleryImageName, "x86_64") {
		return string(compute.ArchitectureX64)
	}
	return string(compute.ArchitectureArm64)
}

func (r *replicateRequest) getImageType() imageType {
	if strings.Contains(r.galleryImageName, "openshift") {
		return imageTypeSNC
	}
	return imageTypeRhelAI
}

func (r *replicateRequest) getGalleryName() string {
	if strings.Contains(r.galleryImageName, "openshift") {
		return sncGalleryName
	}
	return rhelAIGalleryName
}

func (r *replicateRequest) getResourceGroupName() string {
	if strings.Contains(r.galleryImageName, "openshift") {
		return sncGalleryRGName
	}
	return rhelAIGalleryRGName
}

func (r *replicateRequest) getImageVersion() string {
	parts := strings.Split(r.galleryImageName, "-")
	if len(parts) == 4 {
		return parts[2]
	}
	return "latest"
}
