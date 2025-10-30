package azure

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	resources "github.com/pulumi/pulumi-azure-native-sdk/resources/v3"
	"github.com/pulumi/pulumi-azure-native-sdk/storage/v3"
	"github.com/pulumi/pulumi-command/sdk/v3/go/command/local"
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

func uploadVHD(ctx *pulumi.Context, vhdPath string, blobSASUrl pulumi.StringInput, opts pulumi.ResourceOrInvokeOption) (*local.Command, error) {
	azcopyCmd, err := local.NewCommand(ctx, "azcopy-vhd", &local.CommandArgs{
		Create: pulumi.Sprintf("azcopy cp %s '%s' --blob-type='PageBlob'", vhdPath, blobSASUrl),
	}, opts)
	if err != nil {
		return nil, err
	}
	return azcopyCmd, nil
}

func randomize(base string) string {
	r := make([]byte, 4)
	rand.Read(r) // nolint
	return fmt.Sprintf("%s-%x", base, r)
}

func storageAccount(ctx *pulumi.Context) (*storage.BlobContainer, *storage.StorageAccount, *resources.ResourceGroup, error) {
	resourceGroup, err := resources.NewResourceGroup(ctx, randomize("resourceGroupSA"), &resources.ResourceGroupArgs{
		ResourceGroupName: pulumi.String(randomize("resourceGroup")),
	}, pulumi.RetainOnDelete(true))
	if err != nil {
		return nil, nil, nil, err
	}

	storageAcc, err := storage.NewStorageAccount(ctx, "storageAccount", &storage.StorageAccountArgs{
		AccountName:       pulumi.String(randomize("cloudimportersa")),
		AccessTier:        storage.AccessTierHot,
		Kind:              pulumi.String(storage.KindStorageV2),
		ResourceGroupName: resourceGroup.Name,
		Sku: &storage.SkuArgs{
			Name: pulumi.String(storage.SkuName_Premium_LRS),
		},
	}, pulumi.DependsOn([]pulumi.Resource{resourceGroup}))
	if err != nil {
		return nil, nil, nil, err
	}

	blobContainer, err := storage.NewBlobContainer(ctx, "blobContainer", &storage.BlobContainerArgs{
		AccountName:       storageAcc.Name,
		ContainerName:     pulumi.String(randomize("blobbox")),
		ResourceGroupName: resourceGroup.Name,
	}, pulumi.DependsOn([]pulumi.Resource{resourceGroup, storageAcc}))
	if err != nil {
		return nil, nil, nil, err
	}
	return blobContainer, storageAcc, resourceGroup, nil
}

func storageAccSAS(ctx *pulumi.Context, storageAccName, rgName pulumi.StringInput) storage.ListStorageAccountSASResultOutput {
	sasPermissions := pulumi.String(storage.PermissionsW + storage.PermissionsR)
	sasServices := pulumi.String(storage.ServicesB)
	sasResourceTypes := pulumi.String(storage.SignedResourceTypesO)
	sasExpiry := pulumi.String(time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339Nano))
	sasProtocol := storage.HttpProtocolHttps

	sas := storage.ListStorageAccountSASOutput(ctx, storage.ListStorageAccountSASOutputArgs{
		AccountName:            storageAccName,
		ResourceGroupName:      rgName,
		Permissions:            sasPermissions,
		Services:               sasServices,
		ResourceTypes:          sasResourceTypes,
		SharedAccessExpiryTime: sasExpiry,
		Protocols:              &sasProtocol,
	})
	return sas
}

func blobName() string {
	b := make([]byte, 4)
	rand.Read(b) // nolint
	id := fmt.Sprintf("cloud-importer-blobVhd%x.vhd", b)
	return id
}
