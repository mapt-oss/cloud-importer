package manager

import (
	"fmt"

	providerAPI "github.com/mapt-oss/cloud-importer/pkg/manager/provider/api"
	"github.com/mapt-oss/cloud-importer/pkg/provider/aws"
	"github.com/mapt-oss/cloud-importer/pkg/provider/azure"
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
