package api

import (
	"github.com/mapt-oss/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Stack struct {
	ProjectName         string
	StackName           string
	BackedURL           string
	DeployFunc          pulumi.RunFunc
	ProviderCredentials credentials.ProviderCredentials
}

type Provider interface {
	// Manage ephemeral assets
	RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc
	// Manage ephemeral assets
	SNCEphemeral(bundleURI, shasumURI, arch string) pulumi.RunFunc
	// Register AMI and keep state
	ImageRegister(ephemeralResults auto.UpResult, replicate bool, shareOrgIds []string) (pulumi.RunFunc, error)
	// Manage Provider Credentials
	GetProviderCredentials(customCreds map[string]string) credentials.ProviderCredentials
	// Check if an image with the given name exists
	// Returns true and the image identifier if found, false and empty string otherwise
	ImageExists(imageName string) (bool, string, error)
}
