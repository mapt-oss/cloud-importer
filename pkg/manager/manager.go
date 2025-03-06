package manager

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	providerAPI "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/api"
)

const (
	stackImportRHELAIImage string = "importrhelai"
	stackShareImage        string = "shareimage"
)

func ImportRHELAI(ctx *context.ContextArgs,
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
	importFunc, err := p.ImportRHELAI(rawImageFilepath, amiName)
	if err != nil {
		return err
	}
	// Create a stack based on the import function and create it
	stack := providerAPI.Stack{
		// TODO add random ID
		ProjectName: context.ProjectName(),
		StackName:   stackImportRHELAIImage,
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
	shareFunc, err := p.ShareImage(imageID, targetAccountID)
	if err != nil {
		return err
	}
	// Create a stack based on the import function and create it
	stack := providerAPI.Stack{
		// TODO add random ID
		ProjectName: context.ProjectName(),
		StackName:   stackShareImage,
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
