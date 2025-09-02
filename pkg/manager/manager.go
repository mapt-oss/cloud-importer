package manager

import (
	"fmt"
	"sync"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	providerAPI "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/api"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
)

const (
	stackRHELAI             string = "rhelai"
	stackOpenshiftLocal     string = "openshiftloca"
	stackShare              string = "share"
	stackReplicate          string = "replicate"
	azureReplicateStackName string = "az-replicate"

	// aws provider pulumi env
	CONFIG_AWS_REGION string = "aws:region"

	CONFIG_AZURE_LOCATION string = "azure-native:location"
)

func getUniqueStackNameForReplicate(prefix, region string) string {
	return fmt.Sprintf("replicate-%s-%s", prefix, region)
}

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

func OpenshiftLocal(ctx *context.ContextArgs,
	bundleURL string, shasumURL string, arch string,
	regions []string, provider Provider) error {
	// Initialize context
	context.Init(ctx)
	// Get provider
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	importFunc, err := p.OpenshiftLocal(bundleURL, shasumURL, arch, regions)
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

func ReplicateImage(ctx *context.ContextArgs, imageID string, targetRegions []string, provider Provider) error {
	// Initialize context
	context.Init(ctx)
	// Get provider
	var err error

	p, err := getProvider(provider)
	if err != nil {
		return err
	}

	replicateFunc, regions, err := p.Replicate(imageID, targetRegions)
	if err != nil {
		return err
	}

	if provider == AZURE {
		stack := providerAPI.Stack{
			ProjectName: context.ProjectName(),
			StackName:   azureReplicateStackName,
			BackedURL:   context.BackedURL(),
			DeployFunc:  replicateFunc,
		}
		_, err = upStack(stack)
		return err
	}

	var wg sync.WaitGroup
	for _, region := range regions {
		wg.Add(1)
		go func(ctx *context.ContextArgs, amiName, region string) {
			stack := providerAPI.Stack{
				// TODO add random ID
				ProjectName: context.ProjectName(),
				StackName:   getUniqueStackNameForReplicate(context.ProjectName(), region),
				BackedURL:   context.BackedURL(),
				ProviderCredentials: p.GetProviderCredentials(
					map[string]string{
						CONFIG_AWS_REGION: region,
					}),
				DeployFunc: replicateFunc,
			}

			_, err = upStack(stack)
			if err != nil {
				logging.Debugf("Error while trying to replicate to: %s: %v", region, err)
			}
			wg.Done()
		}(ctx, imageID, region)
	}
	wg.Wait()
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
