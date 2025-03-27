package aws

type AMIArch string

var (
	X86   AMIArch = "x86_64"
	ARM64 AMIArch = "arm64"
)

type aws struct{}

func Provider() *aws {
	return &aws{}
}
