package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/storage/v1"

	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
)

func parseGCSBackendURL(backedURL string) (bucket, prefix string, err error) {
	if !strings.HasPrefix(backedURL, "gs://") {
		return "", "", fmt.Errorf("not a GCS backend URL: %s", backedURL)
	}
	gcsPath := strings.TrimPrefix(backedURL, "gs://")
	parts := strings.SplitN(gcsPath, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return "", "", fmt.Errorf("invalid GCS backend URL format: %s", backedURL)
	}
	bucket = parts[0]
	if len(parts) == 2 {
		prefix = parts[1]
	}
	return bucket, prefix, nil
}

const gcsPulumiLocksPath = ".pulumi/locks"

// DeleteLocks removes Pulumi lock files from the GCS backend.
func DeleteLocks(backedURL string) {
	logging.Infof("Force destroy: removing Pulumi locks from %s", backedURL)

	bucket, prefix, err := parseGCSBackendURL(backedURL)
	if err != nil {
		logging.Warnf("Force destroy: failed to parse GCS backend URL: %v", err)
		return
	}

	lockPrefix := gcsPulumiLocksPath + "/"
	if prefix != "" {
		lockPrefix = prefix + "/" + lockPrefix
	}

	if err := gcsDeletePrefix(bucket, lockPrefix); err != nil {
		logging.Warnf("Force destroy: failed to remove locks from GCS: %v", err)
		return
	}

	logging.Info("Force destroy: successfully removed Pulumi locks from GCS")
}

// CleanupState removes Pulumi state files from the GCS backend.
func CleanupState(backedURL string) {
	logging.Infof("Cleaning up Pulumi state from %s", backedURL)

	bucket, prefix, err := parseGCSBackendURL(backedURL)
	if err != nil {
		logging.Warnf("Failed to parse GCS backend URL: %v", err)
		return
	}

	statePrefix := prefix + "/"
	if prefix == "" {
		statePrefix = ""
	}

	if err := gcsDeletePrefix(bucket, statePrefix); err != nil {
		logging.Warnf("Failed to cleanup GCS state: %v", err)
		return
	}

	logging.Info("Successfully cleaned up Pulumi state from GCS")
}

// gcsDeletePrefix deletes all objects in bucket whose name starts with prefix.
func gcsDeletePrefix(bucket, prefix string) error {
	ctx := context.Background()

	var opts []option.ClientOption
	if credJSON := os.Getenv("GOOGLE_CREDENTIALS"); credJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(credJSON)))
	}

	svc, err := storage.NewService(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}

	var deleteErr error
	err = svc.Objects.List(bucket).Prefix(prefix).Pages(ctx, func(page *storage.Objects) error {
		for _, obj := range page.Items {
			if err := svc.Objects.Delete(bucket, obj.Name).Context(ctx).Do(); err != nil {
				deleteErr = fmt.Errorf("failed to delete gs://%s/%s: %w", bucket, obj.Name, err)
				return deleteErr
			}
			logging.Debugf("Deleted gs://%s/%s", bucket, obj.Name)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return deleteErr
}
