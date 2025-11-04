package cmd

import (
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/context"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	awsCMD             string = "aws"
	azureCMD           string = "az"
	projectName        string = "project-name"
	projectNameDesc    string = "project name to identify the execution"
	backedURL          string = "backed-url"
	backedURLDesc      string = "backed for stack state. (local) file:///path/subpath (s3) s3://existing-bucket, (azure) azblob://existing-blobcontainer. See more https://www.pulumi.com/docs/iac/concepts/state-and-backends/#using-a-self-managed-backend"
	debug              string = "debug"
	debugDesc          string = "Enable debug traces and set verbosity to max. Typically to get information to troubleshooting an issue."
	debugLevel         string = "debug-level"
	debugLevelDefault  uint   = 3
	debugLevelDesc     string = "Set the level of verbosity on debug. You can set from minimum 1 to max 9."
	paramReplicate     string = "replicate"
	paramReplicateDesc string = "Provide a list of location to replicate or 'all' to replicate to all available locations"
	paramOrgId         string = "org-id"
	paramOrgIdDesc     string = "Organization identifier to share images"
)

func contextArgsFlags(fs *pflag.FlagSet) {
	fs.StringP(projectName, "", "", projectNameDesc)
	fs.StringP(backedURL, "", "", backedURLDesc)
	fs.Bool(debug, false, debugDesc)
	fs.Uint(debugLevel, debugLevelDefault, debugLevelDesc)
}

func imageControlFlags(fs *pflag.FlagSet) {
	fs.Bool(paramReplicate, false, paramReplicateDesc)
	fs.StringP(paramOrgId, "", "", paramOrgIdDesc)
}

func contextArgs() *context.ContextArgs {
	return &context.ContextArgs{
		ProjectName: viper.GetString(projectName),
		BackedURL:   viper.GetString(backedURL),
		Debug:       viper.IsSet(debug),
		DebugLevel:  viper.GetUint(debugLevel),
	}
}

func imageControl() *manager.ImageControl {
	return &manager.ImageControl{
		Replicate: viper.IsSet(paramReplicate),
		OrgId:     viper.GetString(paramOrgId)}
}
