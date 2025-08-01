package api

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// const (
// 	OutputImageID   string = "ami-image-id"
// 	OutputImageName string = "ami-name"
// )

type Stack struct {
	ProjectName string
	StackName   string
	BackedURL   string
	DeployFunc  pulumi.RunFunc
}

type Provider interface {
	RHELAI(rawImageFilePath, amiName string) (pulumi.RunFunc, error)
	RHELAIOnAzure(subscriptionID, resourceGroup, location, diskPath string, imageName string, tags map[string]string) (pulumi.RunFunc, error)
	Share(imageID string, targetAccountID string) (pulumi.RunFunc, error)
	OpenshiftLocal(bundleURL, shasumURL, arch string) (pulumi.RunFunc, error)
}
