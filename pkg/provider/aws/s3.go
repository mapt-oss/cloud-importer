package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/devtools-qe-incubator/cloud-importer/pkg/util/logging"
)

// parseS3BackendURL parses an S3 backend URL in the format s3://bucket-name/path/to/state
// Returns bucket and key prefix, or an error if the URL is not a valid S3 URL
func parseS3BackendURL(backedURL string) (bucket, key string, err error) {
	if !strings.HasPrefix(backedURL, "s3://") {
		return "", "", fmt.Errorf("not an S3 backend URL: %s", backedURL)
	}

	// Remove s3:// prefix
	s3Path := strings.TrimPrefix(backedURL, "s3://")

	// Split into bucket and key
	parts := strings.SplitN(s3Path, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid S3 backend URL format: %s", backedURL)
	}

	bucket = parts[0]
	key = ""
	if len(parts) == 2 {
		key = parts[1]
	}

	return bucket, key, nil
}

// CleanupState removes Pulumi state files from S3 backend after a successful destroy operation.
// This function logs warnings on errors but does not return errors, as state cleanup failures
// should not fail the destroy operation (resources are already destroyed).
func CleanupState(backedURL string) {
	logging.Infof("Cleaning up Pulumi state from %s", backedURL)

	// Parse the S3 backend URL
	bucket, key, err := parseS3BackendURL(backedURL)
	if err != nil {
		logging.Warnf("Failed to parse S3 backend URL: %v", err)
		return
	}

	// Delete the state from S3
	err = Delete(&bucket, &key)
	if err != nil {
		logging.Warnf("Failed to cleanup S3 state: %v", err)
		return
	}

	logging.Info("Successfully cleaned up Pulumi state from S3")
}

// Delete removes an object or folder (recursively) from S3
func Delete(bucket, key *string) error {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	// Ensure the key ends with "/" to prevent matching sibling directories
	// e.g., "myproject" -> "myproject/" so we don't also match "myproject2" or "myproject-backup"
	normalizedKey := *key
	if normalizedKey != "" && !strings.HasSuffix(normalizedKey, "/") {
		normalizedKey = normalizedKey + "/"
	}

	return deleteRecursive(s3.NewFromConfig(cfg), bucket, &normalizedKey)
}

// deleteRecursive recursively deletes objects from S3
func deleteRecursive(client *s3.Client, bucket, key *string) error {
	isFolder, err := isFolder(client, bucket, key)
	if err != nil {
		return err
	}
	if !isFolder {
		_, err = client.DeleteObject(
			context.Background(),
			&s3.DeleteObjectInput{
				Bucket: bucket,
				Key:    key,
			})
		return err
	}
	// Recursive: delete all children
	childrenKeys, err := listObjectKeys(client, bucket, key)
	if err != nil {
		return err
	}
	for _, cKey := range childrenKeys {
		err = deleteRecursive(client, bucket, &cKey)
		if err != nil {
			logging.Error(err)
		}
	}
	return nil
}

// isFolder checks if the key represents a folder (prefix) in S3
func isFolder(client *s3.Client, bucket, key *string) (bool, error) {
	// If key is empty or ends with /, it's a folder
	if key == nil || *key == "" || strings.HasSuffix(*key, "/") {
		return true, nil
	}

	// Try to list objects with this prefix to determine if it's a folder
	maxKeys := int32(1)
	delimiter := "/"
	result, err := client.ListObjectsV2(
		context.Background(),
		&s3.ListObjectsV2Input{
			Bucket:    bucket,
			Prefix:    key,
			MaxKeys:   &maxKeys,
			Delimiter: &delimiter,
		})
	if err != nil {
		return false, err
	}

	// If we have common prefixes or more than just the exact key, it's a folder
	return len(result.CommonPrefixes) > 0 || len(result.Contents) > 1, nil
}

// listObjectKeys lists all object keys under a prefix
func listObjectKeys(client *s3.Client, bucket, prefix *string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: bucket,
		Prefix: prefix,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			keys = append(keys, *obj.Key)
		}
	}

	return keys, nil
}
