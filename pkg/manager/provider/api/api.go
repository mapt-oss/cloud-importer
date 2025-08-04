package api

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/redhat-developer/mapt/pkg/manager/credentials"
)

// const (
// 	OutputImageID   string = "ami-image-id"
// 	OutputImageName string = "ami-name"
// )

type Stack struct {
	ProjectName         string
	StackName           string
	BackedURL           string
	DeployFunc          pulumi.RunFunc
	ProviderCredentials credentials.ProviderCredentials
}

type Provider interface {
	RHELAI(rawImageFilePath, amiName string) (pulumi.RunFunc, error)
	Share(imageID string, targetAccountID string) (pulumi.RunFunc, error)
	OpenshiftLocal(bundleURL, shasumURL, arch string) (pulumi.RunFunc, error)
	Replicate(amiName string, targetRegions []string) (pulumi.RunFunc, error)
	GetProviderCredentials(customCreds map[string]string) credentials.ProviderCredentials
}
