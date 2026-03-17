package aws

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
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
	// emptyBucket ensures the bucket is cleaned up on destroy even if the upload
	// command state is lost (e.g. process crash during upload). It removes all
	// objects and aborts any incomplete multipart uploads before the bucket is
	// deleted by Pulumi.
	_, err = local.NewCommand(ctx, "emptyBucket", &local.CommandArgs{
		Delete: pulumi.String(fmt.Sprintf(
			"aws s3 rm s3://%s/ --recursive --only-show-error 2>/dev/null || true; "+
				"aws s3api list-multipart-uploads --bucket %s --query 'Uploads[*].[Key,UploadId]' "+
				"--output text 2>/dev/null | "+
				"xargs -r -n2 sh -c 'aws s3api abort-multipart-upload --bucket %s --key \"$1\" --upload-id \"$2\" 2>/dev/null' sh; "+
				"exit 0",
			*bucketName, *bucketName, *bucketName)),
	}, pulumi.DependsOn([]pulumi.Resource{b}))
	if err != nil {
		return err
	}
	ro, _, err := createVMIEmportExportRole(ctx, bucketName)
	if err != nil {
		return err
	}
	ctx.Export(outRoleName, ro.RoleName)
	_, err = uploadDisk(ctx, &r.rawImageFilePath, bucketName, []pulumi.Resource{b, ro})
	return err
}
