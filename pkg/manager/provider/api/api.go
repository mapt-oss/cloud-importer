package api

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
	OpenshiftLocal(bundleURL, shasumURL, arch string, targeRegions []string) (pulumi.RunFunc, error)
	Replicate(amiName string, targetRegions []string) (pulumi.RunFunc, []string, error)
	GetProviderCredentials(customCreds map[string]string) credentials.ProviderCredentials
}
