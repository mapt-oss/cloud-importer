package constants

import "github.com/spf13/pflag"

const (
	BackedURL         string = "backed-url"
	BackedURLDesc     string = "backed for stack state. (local) file:///path/subpath (s3) s3://existing-bucket, (azure) azblob://existing-blobcontainer. See more https://www.pulumi.com/docs/iac/concepts/state-and-backends/#using-a-self-managed-backend"
	Output            string = "output"
	OutputDesc        string = "path to export information regarding the cloud image"
	Debug             string = "debug"
	DebugDesc         string = "Enable debug traces and set verbosity to max. Typically to get information to troubleshooting an issue."
	DebugLevel        string = "debug-level"
	DebugLevelDefault uint   = 3
	DebugLevelDesc    string = "Set the level of verbosity on debug. You can set from minimum 1 to max 9."
)

func AddCommonFlags(fs *pflag.FlagSet) {
	fs.StringP(BackedURL, "", "", BackedURLDesc)
	fs.Bool(Debug, false, DebugDesc)
	fs.Uint(DebugLevel, DebugLevelDefault, DebugLevelDesc)
}
