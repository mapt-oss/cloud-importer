package api

import (
	"context"

	"github.com/mapt-oss/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Stack struct {
	ProjectName         string
	StackName           string
	BackedURL           string
	DeployFunc          pulumi.RunFunc
	ProviderCredentials credentials.ProviderCredentials
	// ProgressMonitor, if set, is run in a goroutine for the duration of the
	// stack update and cancelled when the update completes.
	ProgressMonitor func(ctx context.Context)
}

type Provider interface {
	// Manage ephemeral assets
	RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc
	// Manage ephemeral assets
	SNCEphemeral(bundleURI, shasumURI, arch string) pulumi.RunFunc
	// ValidateShareTargets checks share target identifiers for errors (e.g. duplicates)
	// before any upload or resource creation begins. Returns an error if the targets are invalid.
	ValidateShareTargets(shareOrgIds []string) error
	// Register AMI and keep state.
	// The returned ProgressMonitor (if non-nil) is run in a goroutine for the
	// duration of the stack update to report long-running operation progress.
	ImageRegister(ephemeralResults auto.UpResult, replicate bool, shareOrgIds []string) (pulumi.RunFunc, func(ctx context.Context), error)
	// Manage Provider Credentials
	GetProviderCredentials(customCreds map[string]string) credentials.ProviderCredentials
	// Check if an image with the given name exists
	// Returns true and the image identifier if found, false and empty string otherwise
	ImageExists(imageName string) (bool, string, error)
	// Delete Pulumi lock files from the backend storage
	DeleteLocks(backedURL string)
	// Clean up Pulumi state files from the backend storage
	CleanupState(backedURL string)
}
