package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	ac "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	providerAPI "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/api"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/credentials"
	awsprovider "github.com/devtools-qe-incubator/cloud-importer/pkg/provider/aws"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type ManagerOptions struct {
	// This option informs the manager the actions will be run on background
	// through a routine so in that case we can not return exit but an error
	Baground bool
}

func upStack(targetStack providerAPI.Stack, opts ...ManagerOptions) (auto.UpResult, error) {
	return upStackTargets(targetStack, nil, opts...)
}

func upStackTargets(targetStack providerAPI.Stack, targetURNs []string, opts ...ManagerOptions) (auto.UpResult, error) {
	logging.Debugf("managing stack %s", targetStack.StackName)
	ctx := context.Background()
	objectStack := getStack(ctx, targetStack)
	w := logging.GetWritter()
	defer func() {
		if err := w.Close(); err != nil {
			logging.Error(err)
		}
	}()
	mOpts := []optup.Option{
		optup.ProgressStreams(w),
	}
	if ac.Debug() {
		dl := ac.DebugLevel()
		mOpts = append(mOpts, optup.DebugLogging(
			debug.LoggingOptions{
				LogLevel:      &dl,
				Debug:         true,
				FlowToPlugins: true,
				LogToStdErr:   true}))
	}
	if len(targetURNs) > 0 {
		mOpts = append(mOpts, optup.Target(targetURNs))
	}
	r, err := objectStack.Up(ctx, mOpts...)
	if err != nil {
		logging.Error(err)
		if len(opts) == 1 && opts[0].Baground {
			return auto.UpResult{}, err
		}
		os.Exit(1)
	}
	return r, nil
}

func destroyStack(targetStack providerAPI.Stack, opts ...ManagerOptions) (err error) {
	logging.Debugf("destroying stack %s", targetStack.StackName)
	ctx := context.Background()
	objectStack := getStack(ctx, targetStack)
	w := logging.GetWritter()
	defer func() {
		if err := w.Close(); err != nil {
			logging.Error(err)
		}
	}()
	// stdoutStreamer := optdestroy.ProgressStreams(w)
	mOpts := []optdestroy.Option{
		optdestroy.ProgressStreams(w),
	}
	if ac.Debug() {
		dl := ac.DebugLevel()
		mOpts = append(mOpts, optdestroy.DebugLogging(
			debug.LoggingOptions{
				LogLevel:      &dl,
				FlowToPlugins: true,
				LogToStdErr:   true}))
	}
	if _, err := objectStack.Destroy(ctx, mOpts...); err != nil {
		logging.Error(err)
		os.Exit(1)
	}
	if err := objectStack.Workspace().RemoveStack(ctx, targetStack.StackName); err != nil {
		logging.Error(err)
		if len(opts) == 1 && opts[0].Baground {
			return err
		}
		os.Exit(1)
	}

	// Cleanup Pulumi state from S3 backend after successful destroy
	if !ac.KeepState() && strings.HasPrefix(targetStack.BackedURL, "s3://") {
		awsprovider.CleanupState(targetStack.BackedURL)
	}

	return nil
}

// this function gets our stack ready for update/destroy by prepping the workspace, init/selecting the stack
// and doing a refresh to make sure state and cloud resources are in sync
func getStack(ctx context.Context, target providerAPI.Stack) auto.Stack {
	// create or select a stack with an inline Pulumi program
	s, err := auto.UpsertStackInlineSource(ctx, target.StackName,
		target.ProjectName, target.DeployFunc, getOpts(target)...)
	if err != nil {
		logging.Errorf("Failed to create or select stack: %v", err)
		os.Exit(1)
	}
	if err = postStack(ctx, target, &s); err != nil {
		logging.Error(err)
		os.Exit(1)
	}
	return s
}

func getOpts(target providerAPI.Stack) []auto.LocalWorkspaceOption {
	return []auto.LocalWorkspaceOption{
		auto.Project(workspace.Project{
			Name:    tokens.PackageName(target.ProjectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{
				URL: target.BackedURL,
			},
		}),
		auto.WorkDir(filepath.Join(".")),
		// auto.SecretsProvider("awskms://alias/pulumi-secret-encryption"),
	}
}

func postStack(ctx context.Context, target providerAPI.Stack, stack *auto.Stack) (err error) {
	// Set credentials
	if err = credentials.SetProviderCredentials(ctx, stack, target.ProviderCredentials); err != nil {
		return
	}
	_, err = stack.Refresh(ctx)
	return
}

// func writeOutputs(stackResult auto.UpResult,
// 	destinationFolder string, results map[string]string) (err error) {
// 	for k, v := range results {
// 		if err = writeOutput(stackResult, k, destinationFolder, v); err != nil {
// 			return err
// 		}
// 	}
// 	return
// }

// func writeOutput(stackResult auto.UpResult, outputkey,
// 	destinationFolder, destinationFilename string) error {
// 	value, ok := stackResult.Outputs[outputkey].Value.(string)
// 	if ok {
// 		err := os.WriteFile(path.Join(destinationFolder, destinationFilename), []byte(value), 0600)
// 		if err != nil {
// 			return err
// 		}
// 	} else {
// 		return fmt.Errorf("output value %s not found", outputkey)
// 	}
// 	return nil
// }
