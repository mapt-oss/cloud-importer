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
	ImportRHELAI(rawImageFilePath, amiName string) (pulumi.RunFunc, error)
	ShareImage(imageID string, targetAccountID string) (pulumi.RunFunc, error)
}
