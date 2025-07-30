package azure

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type azureProvider struct{}

func Provider() *azureProvider {
	return &azureProvider{}
}

func (p *azureProvider) RHELAI(rawImageFilePath, amiName string) (pulumi.RunFunc, error) {
	rhelAIReq := rhelAIRequest{
		vhdPath:   rawImageFilePath,
		imageName: amiName,
	}
	return rhelAIReq.runFunc, nil
}

func (p *azureProvider) Share(imageID string, targetAccountID string) (pulumi.RunFunc, error) {
	// Not implemented for Azure
	return nil, nil
}

func (p *azureProvider) OpenshiftLocal(bundleURL, shasumURL, arch string) (pulumi.RunFunc, error) {
	// Not implemented for Azure
	return nil, nil
}
