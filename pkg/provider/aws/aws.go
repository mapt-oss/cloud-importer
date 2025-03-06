package aws

type Provider struct{}

func GetProvider() *Provider {
	return &Provider{}
}
