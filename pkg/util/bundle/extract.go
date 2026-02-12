package bundle

import (
	_ "embed"
	"os"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/util"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var ExtractedRAWDiskFileName = "disk.raw"
var ExtractedVHDDiskFileName = "disk.vhd"

//go:embed extract.sh
var script []byte

func Extract(ctx *pulumi.Context, imageName, bundleURI, shasumURI, provider string) (*local.Command, error) {
	fullFilePath, err := util.WriteTempFile(&imageName, string(script))
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(*fullFilePath, 0777); err != nil {
		return nil, err
	}
	execScriptENVS := map[string]string{
		"BUNDLE_DOWNLOAD_URL":     bundleURI,
		"SHASUMFILE_DOWNLOAD_URL": shasumURI,
		"CLOUD_PROVIDER":          provider}
	return local.NewCommand(ctx,
		"execExtractScript",
		&local.CommandArgs{
			Create:      pulumi.String(*fullFilePath),
			Environment: pulumi.ToStringMap(execScriptENVS),
		},
		pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "20m",
			Update: "20m",
			Delete: "20m",
		}))

}
