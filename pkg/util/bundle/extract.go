package bundle

import (
	_ "embed"
	"os"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var ExtractedDiskFileName = "disk.raw"

//go:embed extract.sh
var script []byte

func Extract(ctx *pulumi.Context, bundleURL, shasumURL string) (*local.Command, error) {
	// Write to temp file to be executed locally
	scriptfileName, err := util.WriteTempFile(string(script))
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(*scriptfileName, 0777); err != nil {
		return nil, err
	}
	execScriptENVS := map[string]string{
		"BUNDLE_DOWNLOAD_URL":     bundleURL,
		"SHASUMFILE_DOWNLOAD_URL": shasumURL}
	return local.NewCommand(ctx, "execExtractScript",
		&local.CommandArgs{
			Create:      pulumi.String(*scriptfileName),
			Environment: pulumi.ToStringMap(execScriptENVS),
		},
		pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "20m",
			Update: "20m",
			Delete: "20m",
		}))

}
