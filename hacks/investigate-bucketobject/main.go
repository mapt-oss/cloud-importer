// investigate-bucketobject tests the two-step upload approach:
// 1. local.Command creates a file (simulating compression)
// 2. storage.NewBucketObject uploads it using only GOOGLE_CREDENTIALS
//
// This validates that Pulumi reads the file asset AFTER the local.Command
// dependency runs — not during the resource discovery phase. If ordering
// is wrong, pulumi up will fail with a "file not found" error.
//
// Run:
//
//	GOOGLE_PROJECT=<project> GOOGLE_CREDENTIALS=<sa-key-json> \
//	GOOGLE_REGION=<region> go run ./hacks/investigate-bucketobject/
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-gcp/sdk/v8/go/gcp/storage"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	project := requireEnv("GOOGLE_PROJECT")
	credentials := requireEnv("GOOGLE_CREDENTIALS")
	region := requireEnv("GOOGLE_REGION")

	// Explicitly unset any file-based credential path so the test is clean.
	os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	// Pulumi requires a passphrase for the local secrets manager when any
	// config value is marked Secret. Set a fixed value for this investigation.
	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "investigate")

	testFile := "/tmp/ci-bucketobj-test.txt"

	ctx := context.Background()

	program := func(ctx *pulumi.Context) error {
		bucket, err := storage.NewBucket(ctx, "test-bucket", &storage.BucketArgs{
			Name:                     pulumi.String(fmt.Sprintf("ci-bucketobj-investigation-%s", project)),
			Location:                 pulumi.String(region),
			ForceDestroy:             pulumi.Bool(true),
			UniformBucketLevelAccess: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		// Step 1: local.Command creates the file (simulates compression).
		// The file does NOT exist before pulumi up runs.
		created, err := local.NewCommand(ctx, "create-file", &local.CommandArgs{
			Create: pulumi.String(fmt.Sprintf(
				"echo 'hello from storage.BucketObject — no GOOGLE_APPLICATION_CREDENTIALS' > %s", testFile)),
			Delete: pulumi.String(fmt.Sprintf("rm -f %s", testFile)),
		}, pulumi.DependsOn([]pulumi.Resource{bucket}))
		if err != nil {
			return err
		}

		// Step 2: BucketObject uploads the file created in step 1.
		// Key question: does Pulumi try to read testFile before step 1 runs?
		_, err = storage.NewBucketObject(ctx, "test-object", &storage.BucketObjectArgs{
			Bucket: bucket.Name,
			Name:   pulumi.String("test.txt"),
			Source: pulumi.NewFileAsset(testFile),
		}, pulumi.DependsOn([]pulumi.Resource{created}))
		return err
	}

	stack, err := auto.UpsertStackInlineSource(ctx, "investigate-bucketobject", "investigate-bucketobject", program,
		auto.WorkDir("."),
	)
	must(err, "create stack")

	must(stack.SetConfig(ctx, "gcp:project", auto.ConfigValue{Value: project}), "set project")
	must(stack.SetConfig(ctx, "gcp:credentials", auto.ConfigValue{Value: credentials, Secret: true}), "set credentials")
	must(stack.SetConfig(ctx, "gcp:region", auto.ConfigValue{Value: region}), "set region")

	fmt.Println("--- Running pulumi up ---")
	upRes, err := stack.Up(ctx, optup.ProgressStreams(os.Stdout))
	if err != nil {
		fmt.Printf("\nFAIL: pulumi up error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nSUCCESS: %v — uploaded without GOOGLE_APPLICATION_CREDENTIALS\n", upRes.Summary.Result)

	fmt.Println("\n--- Destroying ---")
	_, err = stack.Destroy(ctx, optdestroy.ProgressStreams(os.Stdout))
	must(err, "destroy")
	fmt.Println("Done.")
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "error: %s must be set\n", key)
		os.Exit(1)
	}
	return v
}

func must(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error (%s): %v\n", msg, err)
		os.Exit(1)
	}
}
