package ibm

import (
	gocontext "context"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/manager/context"
	ibmcloud "github.com/mapt-oss/pulumi-ibmcloud/sdk/go/ibmcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ibmcloudProviderVersion returns the version of the pulumi-ibmcloud SDK as
// recorded in the build's module graph, so it stays in sync with go.mod
// without any hardcoding.
func ibmcloudProviderVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/mapt-oss/pulumi-ibmcloud/sdk" {
			return strings.TrimPrefix(dep.Version, "v")
		}
	}
	return ""
}


func (p *ibmProvider) ImageRegister(ephemeralResults auto.UpResult, _ bool, _ []string) (pulumi.RunFunc, func(gocontext.Context), error) {
	imageNameOutput, ok := ephemeralResults.Outputs[outImageName]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outImageName)
	}
	cosURIOutput, ok := ephemeralResults.Outputs[outCOSURI]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outCOSURI)
	}
	osSlugOutput, ok := ephemeralResults.Outputs[outOSSlug]
	if !ok {
		return nil, nil, fmt.Errorf("output not found: %s", outOSSlug)
	}

	existingPolicyID, err := findVPCCOSAuthPolicy()
	if err != nil {
		// Non-fatal: log and let Pulumi attempt creation (may get 409 on a fresh run).
		existingPolicyID = ""
	}

	r := ibmRegisterRequest{
		imageName:            imageNameOutput.Value.(string),
		cosURI:               cosURIOutput.Value.(string),
		osSlug:               osSlugOutput.Value.(string),
		existingAuthPolicyID: existingPolicyID,
	}
	return r.registerFunc, nil, nil
}

type ibmRegisterRequest struct {
	imageName            string
	cosURI               string
	osSlug               string
	existingAuthPolicyID string
}

func (r *ibmRegisterRequest) registerFunc(ctx *pulumi.Context) error {
	name := sanitizeImageName(r.imageName)

	// Create an explicit provider so that Pulumi uses the exact binary version
	// that matches the SDK in go.mod, rather than whatever is cached first.
	region, err := sourceRegion()
	if err != nil {
		return err
	}
	providerOpts := []pulumi.ResourceOption{}
	if v := ibmcloudProviderVersion(); v != "" {
		providerOpts = append(providerOpts, pulumi.Version(v))
	}
	ibmcloudProvider, err := ibmcloud.NewProvider(ctx, "ibmcloud", &ibmcloud.ProviderArgs{
		IbmcloudApiKey: pulumi.StringPtr(os.Getenv("IBMCLOUD_API_KEY")),
		Region:         pulumi.StringPtr(region),
	}, providerOpts...)
	if err != nil {
		return fmt.Errorf("create IBM Cloud provider: %w", err)
	}
	prov := pulumi.Provider(ibmcloudProvider)

	rgName := sanitizeImageName(context.ProjectName())
	if len(rgName) > 40 {
		rgName = strings.TrimRight(rgName[:40], "-")
	}
	rg, err := ibmcloud.NewResourceGroup(ctx, "resourceGroup", &ibmcloud.ResourceGroupArgs{
		Name: pulumi.String(rgName),
	}, prov)
	if err != nil {
		return err
	}

	// VPC needs a service-to-service IAM authorization to read from COS when importing images.
	// If the policy already exists (e.g. from a previous stack), import it rather than creating.
	authPolicyOpts := []pulumi.ResourceOption{prov, pulumi.DependsOn([]pulumi.Resource{rg})}
	if r.existingAuthPolicyID != "" {
		authPolicyOpts = append(authPolicyOpts, pulumi.Import(pulumi.ID(r.existingAuthPolicyID)))
	}
	authPolicy, err := ibmcloud.NewIamAuthorizationPolicy(ctx, "vpcCosAuth", &ibmcloud.IamAuthorizationPolicyArgs{
		SourceServiceName:  pulumi.String("is"),
		SourceResourceType: pulumi.String("image"),
		TargetServiceName:  pulumi.String("cloud-object-storage"),
		Roles:              pulumi.StringArray{pulumi.String("Reader")},
	}, authPolicyOpts...)
	if err != nil {
		return err
	}

	image, err := ibmcloud.NewIsImage(ctx, "image", &ibmcloud.IsImageArgs{
		Name:            pulumi.String(name),
		Href:            pulumi.String(r.cosURI),
		OperatingSystem: pulumi.String(r.osSlug),
		ResourceGroup:   rg.ID().ToStringPtrOutput(),
		AllowedUse: ibmcloud.IsImageAllowedUsePtrInput(&ibmcloud.IsImageAllowedUseArgs{
			BareMetalServer: pulumi.StringPtr("true"),
			Instance:        pulumi.StringPtr("true"),
		}),
	},
		prov,
		pulumi.DependsOn([]pulumi.Resource{rg, authPolicy}),
		pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "6h",
			Update: "2h",
			Delete: "30m",
		}))
	if err != nil {
		return err
	}

	// Always create the private catalog: it is required for the operator to
	// complete the UI sharing step (VPC → Images → Share to catalog).
	// Account IDs are applied later via `cloud-importer ibm publish`.
	if err := r.registerCatalogSharing(ctx, prov, image); err != nil {
		return err
	}
	return nil
}

func (r *ibmRegisterRequest) registerCatalogSharing(ctx *pulumi.Context, prov pulumi.ResourceOption, image *ibmcloud.IsImage) error {
	catalog, err := ibmcloud.NewCmCatalog(ctx, "catalog", &ibmcloud.CmCatalogArgs{
		Label:            pulumi.StringPtr(context.ProjectName()),
		ShortDescription: pulumi.StringPtr("Private catalog for cloud-importer VPC images"),
	}, prov, pulumi.IgnoreChanges([]string{"kind"}))
	if err != nil {
		return err
	}

	// The catalog offering, version, and account sharing are handled programmatically
	// by PublishVPCImage after the register stack completes (when --catalog-version is set).
	ctx.Export("catalogID", catalog.ID())
	ctx.Export("catalogName", pulumi.String(context.ProjectName()))
	ctx.Export("vpcImageID", image.ID())
	ctx.Export("vpcImageName", pulumi.String(r.imageName))
	ctx.Export(outOfferingName, pulumi.String(sanitizeImageName(r.imageName)))
	return nil
}
