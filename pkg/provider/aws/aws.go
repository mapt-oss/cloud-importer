package aws

import (
	"context"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/redhat-developer/mapt/pkg/manager/credentials"
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
	for configKey, envKey := range envCredentials {
		if value, ok := customCredentials[configKey]; ok {
			if err := stack.SetConfig(ctx, configKey,
				auto.ConfigValue{Value: value}); err != nil {
				return err
			}
		} else {
			if err := stack.SetConfig(ctx, configKey,
				auto.ConfigValue{Value: os.Getenv(envKey)}); err != nil {
				return err
			}
		}
	}
	return nil
}
