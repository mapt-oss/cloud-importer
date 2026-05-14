package gcp

import (
	"fmt"
	"os/exec"
	"strings"

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

	lockURI := fmt.Sprintf("gs://%s/%s/%s/**", bucket, prefix, gcsPulumiLocksPath)
	if prefix == "" {
		lockURI = fmt.Sprintf("gs://%s/%s/**", bucket, gcsPulumiLocksPath)
	}

	if err := gsutilRM(lockURI); err != nil {
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

	stateURI := fmt.Sprintf("gs://%s/%s/**", bucket, prefix)
	if prefix == "" {
		stateURI = fmt.Sprintf("gs://%s/**", bucket)
	}

	if err := gsutilRM(stateURI); err != nil {
		logging.Warnf("Failed to cleanup GCS state: %v", err)
		return
	}

	logging.Info("Successfully cleaned up Pulumi state from GCS")
}

func gsutilRM(uri string) error {
	out, err := exec.Command("gsutil", "rm", "-rf", uri).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gsutil rm -rf %s: %w: %s", uri, err, strings.TrimSpace(string(out)))
	}
	return nil
}
