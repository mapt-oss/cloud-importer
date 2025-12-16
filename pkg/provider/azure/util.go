package azure

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	resources "github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v3"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const ENV_AZURE_SUBSCRIPTION_ID = "ARM_SUBSCRIPTION_ID"

const (
	sasURLBase  = "https://%s.blob.core.windows.net/%s/%s?%s"
	blobURLBase = "https://%s.blob.core.windows.net/%s/%s"
)

func getCredentials() (cred *azidentity.DefaultAzureCredential, subscriptionID *string, err error) {
	cred, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return
	}
	azSubsID := os.Getenv(ENV_AZURE_SUBSCRIPTION_ID)
	subscriptionID = &azSubsID
	return
}

func sliceConvert[T any, Y any](source []Y,
	convert func(item Y) T) []T {
	var result []T
	for _, item := range source {
		result = append(result, convert(item))
	}
	return result
}

func uploadVHD(ctx *pulumi.Context, vhdPath string, blobSASUrl pulumi.StringInput, opts pulumi.ResourceOrInvokeOption) error {
	_, err := local.NewCommand(ctx, "azcopy-vhd", &local.CommandArgs{
		Create: pulumi.Sprintf("azcopy cp %s '%s' --blob-type='PageBlob'", vhdPath, blobSASUrl),
	}, opts)
	return err
}

func sanitizeName(input string) string {
	re := regexp.MustCompile(`[^a-z0-9]`)
	clean := re.ReplaceAllString(input, "")
	if len(clean) > 24 {
		clean = clean[:24]
	}

	return clean
}

func storageAccount(ctx *pulumi.Context, location, name *string) (*storage.BlobContainer, *storage.StorageAccount, *resources.ResourceGroup, error) {
	ephemeralName := sanitizeName(fmt.Sprintf("ephemeral%s", *name))
	resourceGroup, err := resources.NewResourceGroup(ctx,
		"vhd",
		&resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(ephemeralName),
			Location:          pulumi.String(*location),
		})
	if err != nil {
		return nil, nil, nil, err
	}
	storageAcc, err := storage.NewStorageAccount(ctx,
		"vhd",
		&storage.StorageAccountArgs{
			AccountName:       pulumi.String(ephemeralName),
			AccessTier:        storage.AccessTierHot,
			Kind:              pulumi.String(storage.KindStorageV2),
			ResourceGroupName: resourceGroup.Name,
			Sku: &storage.SkuArgs{
				Name: pulumi.String(storage.SkuName_Premium_LRS),
			},
			Location: pulumi.String(*location),
		}, pulumi.DependsOn([]pulumi.Resource{resourceGroup}))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error with sa: %s: %v", ephemeralName, err)
	}
	blobContainer, err := storage.NewBlobContainer(
		ctx,
		"vhd",
		&storage.BlobContainerArgs{
			AccountName:       storageAcc.Name,
			ContainerName:     pulumi.String(ephemeralName),
			ResourceGroupName: resourceGroup.Name,
		})
	if err != nil {
		return nil, nil, nil, err
	}
	return blobContainer, storageAcc, resourceGroup, nil
}

func storageAccSAS(ctx *pulumi.Context, storageAccName, rgName pulumi.StringInput) storage.ListStorageAccountSASResultOutput {
	sasProtocol := storage.HttpProtocolHttps
	now := time.Now().UTC()
	fixedExpiryDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
	sas := storage.ListStorageAccountSASOutput(ctx,
		storage.ListStorageAccountSASOutputArgs{
			AccountName:            storageAccName,
			ResourceGroupName:      rgName,
			Permissions:            pulumi.String(storage.PermissionsW + storage.PermissionsR),
			Services:               pulumi.String(storage.ServicesB),
			ResourceTypes:          pulumi.String(storage.SignedResourceTypesO),
			SharedAccessExpiryTime: pulumi.String(fixedExpiryDate.Format(time.RFC3339)),
			Protocols:              &sasProtocol,
		})
	return sas
}
