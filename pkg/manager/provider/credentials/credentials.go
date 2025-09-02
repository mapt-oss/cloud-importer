package credentials

import (
	"context"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

type SetCredentialsFunc func(ctx context.Context, stack auto.Stack, fixedCredentials map[string]string) error

type ProviderCredentials struct {
	SetCredentialFunc SetCredentialsFunc
	FixedCredentials  map[string]string
}

func SetProviderCredentials(ctx context.Context, stack *auto.Stack, p ProviderCredentials) (err error) {
	// Set credentials
	if p.SetCredentialFunc != nil {
		err = p.SetCredentialFunc(ctx, *stack, p.FixedCredentials)
	}
	return
}

func SetCredentials(ctx context.Context, stack auto.Stack, customCredentials, credentialEnvs map[string]string) error {
	for configKey, envKey := range credentialEnvs {
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
