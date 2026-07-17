package ibm

import (
	gocontext "context"
	"fmt"

	ibmcloud "github.com/mapt-oss/pulumi-ibmcloud/sdk/go/ibmcloud"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func (p *ibmProvider) ImageRegister(ephemeralResults auto.UpResult, _ bool, shareAccountIds []string) (pulumi.RunFunc, func(gocontext.Context), error) {
	if err := validateShareAccountIds(shareAccountIds); err != nil {
		return nil, nil, err
	}
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

	r := ibmRegisterRequest{
		imageName:       imageNameOutput.Value.(string),
		cosURI:          cosURIOutput.Value.(string),
		osSlug:          osSlugOutput.Value.(string),
		shareAccountIds: shareAccountIds,
	}
	return r.registerFunc, nil, nil
}

type ibmRegisterRequest struct {
	imageName       string
	cosURI          string
	osSlug          string
	shareAccountIds []string
}

func (r *ibmRegisterRequest) registerFunc(ctx *pulumi.Context) error {
	name := sanitizeImageName(r.imageName)

	args := &ibmcloud.IsImageArgs{
		Name:            pulumi.String(name),
		Href:            pulumi.String(r.cosURI),
		OperatingSystem: pulumi.String(r.osSlug),
	}
	if rg := envResourceGroup(); rg != "" {
		args.ResourceGroup = pulumi.String(rg)
	}

	_, err := ibmcloud.NewIsImage(ctx, "image", args,
		pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "6h",
			Update: "2h",
			Delete: "30m",
		}))
	return err
}
