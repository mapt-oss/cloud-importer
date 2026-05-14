package manager

import (
	"fmt"

	"github.com/mapt-oss/cloud-importer/pkg/manager/context"
	providerAPI "github.com/mapt-oss/cloud-importer/pkg/manager/provider/api"
	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	stackRHELAIEphemeral string = "rhelai-ephemeral"
	stackRHELAI          string = "rhelai"
	stackSNCEphemeral    string = "snc-ephemeral"
	stackSNC             string = "snc"

	// aws provider pulumi env
	CONFIG_AWS_REGION     string = "aws:region"
	CONFIG_AZURE_LOCATION string = "azure-native:location"
)

type ImageControl struct {
	Replicate   bool
	ShareOrgIds []string
	Tags        map[string]string // Cloud provider tags (AWS and Azure)
}

type RHELAIArgs struct {
	ImageFilepath string
	ImageName     string
	ImageControl  *ImageControl
}

func RHELAI(ctx *context.ContextArgs,
	args *RHELAIArgs,
	provider Provider) error {
	// Initialize context
	context.Init(ctx)
	// Set provider-specific tags (if any)
	if args.ImageControl.Tags != nil {
		context.SetTags(args.ImageControl.Tags)
	}
	// Get provider
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	if err := p.ValidateShareTargets(args.ImageControl.ShareOrgIds); err != nil {
		return err
	}

	var (
		ephemeralResults auto.UpResult
		ephemeralStack   providerAPI.Stack
		ranEphemeral     bool
	)
	if args.ImageFilepath == "" {
		deriver, ok := p.(providerAPI.EphemeralDeriver)
		if !ok {
			return fmt.Errorf("--image-path is required: this provider does not support updating an existing image without re-uploading")
		}
		logging.Info("--image-path not provided: skipping upload and updating existing image only")
		ephemeralResults = auto.UpResult{Outputs: deriver.DeriveEphemeralOutputs(args.ImageName)}
	} else {
		ephemeralStack = providerAPI.Stack{
			ProjectName: context.ProjectName(),
			StackName:   stackRHELAIEphemeral,
			BackedURL:   context.BackedURL(),
			DeployFunc:  p.RHELAIEphemeral(args.ImageFilepath, args.ImageName)}
		var err error
		ephemeralResults, err = upStack(ephemeralStack)
		if err != nil {
			return err
		}
		ranEphemeral = true
	}

	registerFunc, err := p.ImageRegister(ephemeralResults,
		args.ImageControl.Replicate, args.ImageControl.ShareOrgIds)
	if err != nil {
		return err
	}
	registerStack := providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   stackRHELAI,
		BackedURL:   context.BackedURL(),
		DeployFunc:  registerFunc}
	_, err = upStack(registerStack)
	if err != nil {
		return err
	}
	if ranEphemeral {
		return destroyStack(ephemeralStack, false)
	}
	return nil
}

type SNCArgs struct {
	BundleURI    string
	ShasumURI    string
	Arch         string
	ImageControl *ImageControl
}

func SNC(ctx *context.ContextArgs, args *SNCArgs, provider Provider) error {
	context.Init(ctx)
	// Set provider-specific tags (if any)
	if args.ImageControl.Tags != nil {
		context.SetTags(args.ImageControl.Tags)
	}
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	if err := p.ValidateShareTargets(args.ImageControl.ShareOrgIds); err != nil {
		return err
	}
	ephemeralStack := providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   stackSNCEphemeral,
		BackedURL:   context.BackedURL(),
		DeployFunc:  p.SNCEphemeral(args.BundleURI, args.ShasumURI, args.Arch)}
	ephemeralResults, err := upStack(ephemeralStack)
	if err != nil {
		return err
	}
	registerFunc, err := p.ImageRegister(ephemeralResults,
		args.ImageControl.Replicate, args.ImageControl.ShareOrgIds)
	if err != nil {
		return err
	}
	registerStack := providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   stackSNC,
		BackedURL:   context.BackedURL(),
		DeployFunc:  registerFunc}
	_, err = upStack(registerStack)
	if err != nil {
		return err
	}
	return destroyStack(ephemeralStack, false)
}

func Destoy(ctx *context.ContextArgs) error {
	// Initialize context
	context.Init(ctx)
	if context.ForceDestroy() {
		deleteLocks(context.BackedURL())
	}
	// Attempt to destroy the ephemeral stack in case it was not cleaned up
	// (e.g. after a failed import). A no-op deploy func is sufficient here
	// since destroy only removes resources already tracked in the stack state.
	ephemeralStack := providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   stackRHELAIEphemeral,
		BackedURL:   context.BackedURL(),
		DeployFunc:  func(ctx *pulumi.Context) error { return nil },
	}
	if err := destroyStack(ephemeralStack, false, ManagerOptions{Baground: true}); err != nil {
		logging.Warnf("Could not destroy ephemeral stack (it may already be gone): %v", err)
	}
	return destroyStack(providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   stackRHELAI,
		BackedURL:   context.BackedURL()}, !context.KeepState())
}

func CheckImageExists(imageName string, provider Provider) (bool, string, error) {
	p, err := getProvider(provider)
	if err != nil {
		return false, "", err
	}
	return p.ImageExists(imageName)
}

func deleteLocks(backedURL string) {
	p, err := getProviderByBackedURL(backedURL)
	if err != nil {
		logging.Debugf("force-destroy: %v, skipping lock deletion", err)
		return
	}
	p.DeleteLocks(backedURL)
}
