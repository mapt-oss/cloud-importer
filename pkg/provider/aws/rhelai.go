package aws

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type importRequest struct {
	rawImageFilePath string
	amiName          string
}

var (
	// Currently this is the only arch supported
	rhelaiArch = "x86_64"
)

func (a *aws) RHELAI(rawImageFilePath, amiName string) (pulumi.RunFunc, error) {
	r := importRequest{
		rawImageFilePath,
		amiName}
	return r.runFunc, nil
}

func (r importRequest) runFunc(ctx *pulumi.Context) error {
	id := randomID()
	_, err := bucketEphemeral(ctx, id)
	if err != nil {
		return err
	}
	ro, _, err := createVMIEmportExportRole(ctx, id)
	if err != nil {
		return err
	}
	u, err := uploadDisk(ctx, &r.rawImageFilePath, id, []pulumi.Resource{ro})
	if err != nil {
		return err
	}
	_, err = registerAMI(ctx, &r.amiName, &rhelaiArch, id, ro, []pulumi.Resource{u})
	if err != nil {
		return err
	}
	return nil
}
