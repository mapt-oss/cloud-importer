package manager

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	providerAPI "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/api"
)

const (
	stackRHELAI         string = "rhelai"
	stackOpenshiftLocal string = "openshiftloca"
	stackShare          string = "share"
)

func RHELAI(ctx *context.ContextArgs,
	rawImageFilepath string,
	amiName string,
	provider Provider) error {
	// Initialize context
	context.Init(ctx)
	// Get provider
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	importFunc, err := p.RHELAI(rawImageFilepath, amiName)
	if err != nil {
		return err
	}
	// Create a stack based on the import function and create it
	stack := providerAPI.Stack{
		// TODO add random ID
		ProjectName: context.ProjectName(),
		StackName:   stackRHELAI,
		BackedURL:   context.BackedURL(),
		DeployFunc:  importFunc}
	_, err = upStack(stack)
	if err != nil {
		return err
	}
	// err = manageImageImportResults(stackResult, context.Output())
	// if err != nil {
	// 	return nil
	// }
	// Current exec create temporary resources to enable the import
	// we delete it as they are only temporary
	return destroyStack(stack)
}

func RHELAIOnAzure(ctx *context.ContextArgs,
	subscriptionID, resourceGroup, location, diskPath string, imageName string, tags map[string]string,
	provider Provider) error {
	// Initialize context
	context.Init(ctx)
	// Get provider
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	importFunc, err := p.RHELAIOnAzure(subscriptionID, resourceGroup, location, diskPath, imageName, tags)
	if err != nil {
		return err
	}
	// Create a stack based on the import function and create it
	stack := providerAPI.Stack{
		// TODO add random ID
		ProjectName: context.ProjectName(),
		StackName:   stackRHELAI,
		BackedURL:   context.BackedURL(),
		DeployFunc:  importFunc}
	_, err = upStack(stack)
	if err != nil {
		return err
	}
	// err = manageImageImportResults(stackResult, context.Output())
	// if err != nil {
	// 	return nil
	// }
	// Current exec create temporary resources to enable the import
	// we delete it as they are only temporary
	return destroyStack(stack)
}

func OpenshiftLocal(ctx *context.ContextArgs,
	bundleURL string, shasumURL string, arch string,
	provider Provider) error {
	// Initialize context
	context.Init(ctx)
	// Get provider
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	importFunc, err := p.OpenshiftLocal(bundleURL, shasumURL, arch)
	if err != nil {
		return err
	}
	// Create a stack based on the import function and create it
	stack := providerAPI.Stack{
		// TODO add random ID
		ProjectName: context.ProjectName(),
		StackName:   stackOpenshiftLocal,
		BackedURL:   context.BackedURL(),
		DeployFunc:  importFunc}
	_, err = upStack(stack)
	if err != nil {
		return err
	}
	// err = manageImageImportResults(stackResult, context.Output())
	// if err != nil {
	// 	return nil
	// }
	// Current exec create temporary resources to enable the import
	// we delete it as they are only temporary
	return destroyStack(stack)
}

func ShareImage(ctx *context.ContextArgs,
	imageID, targetAccountID string,
	provider Provider) error {
	// Initialize context
	context.Init(ctx)
	// Get provider
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	shareFunc, err := p.Share(imageID, targetAccountID)
	if err != nil {
		return err
	}
	// Create a stack based on the import function and create it
	stack := providerAPI.Stack{
		// TODO add random ID
		ProjectName: context.ProjectName(),
		StackName:   stackShare,
		BackedURL:   context.BackedURL(),
		DeployFunc:  shareFunc}
	_, err = upStack(stack)
	if err != nil {
		return err
	}
	return nil
}

// func manageImageImportResults(stackResult auto.UpResult, destinationFolder string) error {
// 	if err := writeOutputs(stackResult, destinationFolder, map[string]string{
// 		providerAPI.OutputBootKey: "id_ecdsa",
// 		providerAPI.OutputImageID: "image-id",
// 	}); err != nil {
// 		return err
// 	}
// 	return nil
// }
