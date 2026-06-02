package gcp

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	storage_api "google.golang.org/api/storage/v1"
)

// StreamUpload uploads src to gs://bucket/object using the GCS JSON API
// resumable upload. For files larger than 8 MB the client library automatically
// uses a chunked resumable upload session rather than loading the file into
// memory, making it safe for large disk images.
// Authentication is read from GOOGLE_CREDENTIALS (inline JSON) if set;
// otherwise Application Default Credentials are used.
func StreamUpload(src, bucket, object string) error {
	ctx := context.Background()

	var opts []option.ClientOption
	if credJSON := os.Getenv("GOOGLE_CREDENTIALS"); credJSON != "" {
		creds, err := google.CredentialsFromJSON(ctx, []byte(credJSON), storage_api.CloudPlatformScope)
		if err != nil {
			return fmt.Errorf("parse GOOGLE_CREDENTIALS: %w", err)
		}
		opts = append(opts, option.WithCredentials(creds))
	}

	svc, err := storage_api.NewService(ctx, opts...)
	if err != nil {
		return fmt.Errorf("create GCS service: %w", err)
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer f.Close()

	_, err = svc.Objects.Insert(bucket, &storage_api.Object{Name: object}).
		Media(f).
		Do()
	if err != nil {
		return fmt.Errorf("upload to gs://%s/%s: %w", bucket, object, err)
	}
	return nil
}
