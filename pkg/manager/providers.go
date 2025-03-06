package manager

import (
	"fmt"

	providerAPI "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/api"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/provider/aws"
)

type Provider string

const (
	AWS Provider = "aws"
	AZ  Provider = "azure"
)

func getProvider(provider Provider) (providerAPI.Provider, error) {
	switch provider {
	case AWS:
		return aws.GetProvider(), nil
	}
	return nil, fmt.Errorf("%s: provider not supported", provider)
}
