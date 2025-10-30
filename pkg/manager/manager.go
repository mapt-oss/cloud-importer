package manager

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	providerAPI "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/api"
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
	Replicate bool
	OrgId     string
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
	// Get provider
	p, err := getProvider(provider)
	if err != nil {
		return err
	}
	ephemeralStack := providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   stackRHELAIEphemeral,
		BackedURL:   context.BackedURL(),
		DeployFunc:  p.RHELAIEphemeral(args.ImageFilepath, args.ImageName)}
	ephemeralResults, err := upStack(ephemeralStack)
	if err != nil {
		return err
	}
	registerFunc, err := p.ImageRegister(ephemeralResults,
		args.ImageControl.Replicate, args.ImageControl.OrgId)
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
	return destroyStack(ephemeralStack)
}

type SNCArgs struct {
	BundleURI    string
	ShasumURI    string
	Arch         string
	ImageControl *ImageControl
}

func SNC(ctx *context.ContextArgs, args *SNCArgs, provider Provider) error {
	context.Init(ctx)
	p, err := getProvider(provider)
	if err != nil {
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
		args.ImageControl.Replicate, args.ImageControl.OrgId)
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
	return destroyStack(ephemeralStack)
}

func Destoy(ctx *context.ContextArgs) error {
	// Initialize context
	context.Init(ctx)
	return destroyStack(providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   stackRHELAI,
		BackedURL:   context.BackedURL()})
}
