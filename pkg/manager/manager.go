package manager

import (
	"fmt"

	"github.com/mapt-oss/cloud-importer/pkg/manager/context"
	providerAPI "github.com/mapt-oss/cloud-importer/pkg/manager/provider/api"
	"github.com/mapt-oss/cloud-importer/pkg/provider/ibm"
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
	Replicate      bool
	ShareOrgIds    []string
	Tags           map[string]string // Cloud provider tags (AWS and Azure)
	CatalogVersion string            // IBM only: semver for catalog publishing (e.g. "4.22.14")
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
	registerFunc, progressMonitor, err := p.ImageRegister(ephemeralResults,
		args.ImageControl.Replicate, args.ImageControl.ShareOrgIds)
	if err != nil {
		return err
	}
	registerStack := providerAPI.Stack{
		ProjectName:     context.ProjectName(),
		StackName:       stackRHELAI,
		BackedURL:       context.BackedURL(),
		DeployFunc:      registerFunc,
		ProgressMonitor: progressMonitor}
	registerResults, err := upStack(registerStack)
	if err != nil {
		return err
	}
	if pub, ok := p.(providerAPI.PostRegisterPublisher); ok {
		if err := pub.PostRegisterPublish(registerResults, args.ImageControl.CatalogVersion, args.ImageControl.ShareOrgIds); err != nil {
			return err
		}
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
	ImageName    string
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
	var (
		ephemeralResults auto.UpResult
		ephemeralStack   providerAPI.Stack
		ranEphemeral     bool
	)
	// If --bundle-uri is provided without --image-name: try to derive the image
	// name from the bundle URI. If the register stack already has state, skip
	// the ephemeral upload and treat it like an --image-name-only run.
	if args.BundleURI != "" && args.ImageName == "" {
		if namer, ok := p.(providerAPI.BundleImageNamer); ok {
			if name, err := namer.ImageNameFromBundle(args.BundleURI, args.ShasumURI, args.Arch); err == nil {
				if stackHasOutputs(stackSNC, context.ProjectName(), context.BackedURL()) {
					logging.Infof("snc: register stack already exists for %q, skipping upload", name)
					args.ImageName = name
					args.BundleURI = ""
				}
			}
		}
	}
	if args.BundleURI == "" {
		deriver, ok := p.(providerAPI.EphemeralDeriver)
		if !ok {
			return fmt.Errorf("--bundle-uri is required: this provider does not support updating an existing image without re-uploading")
		}
		logging.Info("--bundle-uri not provided: skipping upload and updating existing image only")
		ephemeralResults = auto.UpResult{Outputs: deriver.DeriveEphemeralOutputs(args.ImageName)}
	} else {
		ephemeralStack = providerAPI.Stack{
			ProjectName: context.ProjectName(),
			StackName:   stackSNCEphemeral,
			BackedURL:   context.BackedURL(),
			DeployFunc:  p.SNCEphemeral(args.BundleURI, args.ShasumURI, args.Arch)}
		var err error
		ephemeralResults, err = upStack(ephemeralStack)
		if err != nil {
			return err
		}
		ranEphemeral = true
	}
	registerFunc, progressMonitor, err := p.ImageRegister(ephemeralResults,
		args.ImageControl.Replicate, args.ImageControl.ShareOrgIds)
	if err != nil {
		return err
	}
	registerStack := providerAPI.Stack{
		ProjectName:     context.ProjectName(),
		StackName:       stackSNC,
		BackedURL:       context.BackedURL(),
		DeployFunc:      registerFunc,
		ProgressMonitor: progressMonitor}
	registerResults, err := upStack(registerStack)
	if err != nil {
		return err
	}
	if pub, ok := p.(providerAPI.PostRegisterPublisher); ok {
		if err := pub.PostRegisterPublish(registerResults, args.ImageControl.CatalogVersion, args.ImageControl.ShareOrgIds); err != nil {
			return err
		}
	}
	if ranEphemeral {
		return destroyStack(ephemeralStack, false)
	}
	return nil
}

// DestroyType identifies which image workflow's stacks to tear down.
type DestroyType string

const (
	DestroyRHELAI DestroyType = "rhelai"
	DestroySNC    DestroyType = "snc"
)

func Destoy(ctx *context.ContextArgs, imageType string) error {
	context.Init(ctx)
	if context.ForceDestroy() {
		deleteLocks(context.BackedURL())
	}

	var ephemeralStack, registerStack string
	switch DestroyType(imageType) {
	case DestroyRHELAI:
		ephemeralStack, registerStack = stackRHELAIEphemeral, stackRHELAI
	case DestroySNC:
		ephemeralStack, registerStack = stackSNCEphemeral, stackSNC
	default:
		return fmt.Errorf("--type is required: must be %q or %q", DestroyRHELAI, DestroySNC)
	}

	// Ephemeral stack may already be gone after a successful run — best-effort.
	noopDeploy := func(ctx *pulumi.Context) error { return nil }
	if err := destroyStack(providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   ephemeralStack,
		BackedURL:   context.BackedURL(),
		DeployFunc:  noopDeploy,
	}, false, ManagerOptions{Baground: true}); err != nil {
		logging.Warnf("Could not destroy ephemeral stack %s (it may already be gone): %v", ephemeralStack, err)
	}

	// IBM-specific: before Pulumi destroys resources, delete the catalog and
	// force-complete IBM's reclamation process so the VPC image is no longer
	// catalog_offering.managed by the time Pulumi attempts to delete it.
	// See pkg/provider/ibm/destroy.go for the full rationale.
	if context.RawScheme() == "cos" {
		if err := ibm.PreDestroyCleanup(context.ProjectName(), registerStack, context.BackedURL()); err != nil {
			logging.Warnf("IBM pre-destroy cleanup: %v (destroy may still fail)", err)
		}
	}

	return destroyStack(providerAPI.Stack{
		ProjectName: context.ProjectName(),
		StackName:   registerStack,
		BackedURL:   context.BackedURL(),
	}, !context.KeepState())
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
