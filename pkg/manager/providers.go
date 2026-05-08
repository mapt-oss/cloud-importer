package manager

import (
	"fmt"
	"strings"

	providerAPI "github.com/mapt-oss/cloud-importer/pkg/manager/provider/api"
	"github.com/mapt-oss/cloud-importer/pkg/provider/aws"
	"github.com/mapt-oss/cloud-importer/pkg/provider/azure"
	"github.com/mapt-oss/cloud-importer/pkg/provider/gcp"
)

type Provider string

const (
	AWS   Provider = "aws"
	AZURE Provider = "azure"
	GCP   Provider = "gcp"
)

func getProvider(provider Provider) (providerAPI.Provider, error) {
	switch provider {
	case AWS:
		return aws.Provider(), nil
	case AZURE:
		return azure.Provider(), nil
	case GCP:
		return gcp.Provider(), nil
	}
	return nil, fmt.Errorf("%s: provider not supported", provider)
}

func getProviderByBackedURL(backedURL string) (providerAPI.Provider, error) {
	switch {
	case strings.HasPrefix(backedURL, "s3://"):
		return getProvider(AWS)
	case strings.HasPrefix(backedURL, "azblob://"):
		return getProvider(AZURE)
	case strings.HasPrefix(backedURL, "gs://"):
		return getProvider(GCP)
	}
	return nil, fmt.Errorf("unsupported backend URL: %s", backedURL)
}
