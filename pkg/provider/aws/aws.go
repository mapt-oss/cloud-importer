package aws

import (
	"context"
	"fmt"
	"os"

	"github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/credentials"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

type AMIArch string

var (
	X86   AMIArch = "x86_64"
	ARM64 AMIArch = "arm64"
)

const (
	CONFIG_AWS_REGION        string = "aws:region"
	CONFIG_AWS_NATIVE_REGION string = "aws-native:region"
	CONFIG_AWS_ACCESS_KEY    string = "aws:accessKey"
	CONFIG_AWS_SECRET_KEY    string = "aws:secretKey"
)

// pulumi config key : aws env credential
var envCredentials = map[string]string{
	CONFIG_AWS_REGION:        "AWS_DEFAULT_REGION",
	CONFIG_AWS_NATIVE_REGION: "AWS_DEFAULT_REGION",
	CONFIG_AWS_ACCESS_KEY:    "AWS_ACCESS_KEY_ID",
	CONFIG_AWS_SECRET_KEY:    "AWS_SECRET_ACCESS_KEY",
}

type aws struct{}

func Provider() *aws {
	return &aws{}
}

func (p *aws) GetProviderCredentials(customCredentials map[string]string) credentials.ProviderCredentials {
	return credentials.ProviderCredentials{
		SetCredentialFunc: SetAWSCredentials,
		FixedCredentials:  customCredentials}
}

func SetAWSCredentials(ctx context.Context, stack auto.Stack, customCredentials map[string]string) error {
	return credentials.SetCredentials(ctx, stack, customCredentials, envCredentials)
}

func sourceHostingPlace() (*string, error) {
	hp := os.Getenv("AWS_DEFAULT_REGION")
	if len(hp) > 0 {
		return &hp, nil
	}
	hp = os.Getenv("AWS_REGION")
	if len(hp) > 0 {
		return &hp, nil
	}
	return nil, fmt.Errorf("missing default value for AWS Region")
}
