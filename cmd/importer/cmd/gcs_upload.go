package cmd

import (
	"github.com/mapt-oss/cloud-importer/pkg/provider/gcp"
	"github.com/spf13/cobra"
)

func gcsUploadCmd() *cobra.Command {
	var bucket, object, source string

	cmd := &cobra.Command{
		Use:    "gcs-upload",
		Hidden: true,
		Short:  "Stream a local file to a GCS bucket object (used internally by the GCP provider)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return gcp.StreamUpload(source, bucket, object)
		},
	}
	cmd.Flags().StringVar(&bucket, "bucket", "", "GCS bucket name")
	cmd.Flags().StringVar(&object, "object", "", "GCS object name")
	cmd.Flags().StringVar(&source, "source", "", "Local file path to upload")
	_ = cmd.MarkFlagRequired("bucket")
	_ = cmd.MarkFlagRequired("object")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}
