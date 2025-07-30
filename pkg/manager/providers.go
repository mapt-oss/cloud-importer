package manager

import (
	"fmt"

	providerAPI "github.com/devtools-qe-incubator/cloud-importer/pkg/manager/provider/api"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/provider/aws"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/provider/azure"
)

type Provider string

const (
	AWS   Provider = "aws"
	AZURE Provider = "azure"
)

func getProvider(provider Provider) (providerAPI.Provider, error) {
	switch provider {
	case AWS:
		return aws.Provider(), nil
	case AZURE:
		return azure.Provider(), nil
	}
	return nil, fmt.Errorf("%s: provider not supported", provider)
}
