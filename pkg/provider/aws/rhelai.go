package aws

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type rhelaiEphemeralRequest struct {
	rawImageFilePath string
	amiName          string
}

var (
	// Currently this is the only arch supported
	rhelaiArch = "x86_64"
)

func (a *aws) RHELAIEphemeral(imageFilePath, imageName string) pulumi.RunFunc {
	r := rhelaiEphemeralRequest{
		imageFilePath,
		imageName}
	return r.rhelaiEphemeralRunFunc
}

// This func should add all outputs
func (r rhelaiEphemeralRequest) rhelaiEphemeralRunFunc(ctx *pulumi.Context) error {
	ctx.Export(outAMIName, pulumi.String(r.amiName))
	ctx.Export(outAMIArch, pulumi.String(rhelaiArch))
	bucketName := randomID()
	b, err := bucketEphemeral(ctx, bucketName)
	if err != nil {
		return err
	}
	ctx.Export(outBucketName, pulumi.String(*bucketName))
	ro, _, err := createVMIEmportExportRole(ctx, bucketName)
	if err != nil {
		return err
	}
	ctx.Export(outRoleName, ro.RoleName)
	_, err = uploadDisk(ctx, &r.rawImageFilePath, bucketName, []pulumi.Resource{b, ro})
	return err
}
