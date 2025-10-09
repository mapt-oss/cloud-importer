# OpenShift Local Bundles Upload Guide

The `cloud-importer` tool can be used to upload an OpenShift Local bundle to your AWS or Azure account. It automates the steps of extracting the disk image from the bundle
and making it ready for use as an AMI on AWS or Disk Image on Azure.

## Prerequisites

Before you begin, ensure you have the following:

  * **Cloud Account:** An active AWS or Azure account
  * **Cloud Credentials:**
      * **AWS Credentials**: Your `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_DEFAULT_REGION` must be configured as environment variables.
      * **Azure Credentials**: Your `ARM_CLIENT_ID`, `ARM_CLIENT_SECRET`, `ARM_SUBSCRIPTION_ID` and `ARM_LOCATION_NAME` must be configured as environment variables.
  * **OpenShift Local Bundle:** You'll need the URL for the OpenShift Local bundle and its corresponding checksum URL.

## Usage

### To upload an OpenShift Local bundle to **AWS**, run the following command, replacing the placeholder values with your specific information:

```bash
podman run --rm --name import-openshift-local -d \
    -v ${PWD}:/workspace:z \
    -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
    ghcr.io/mapt-oss/cloud-importer:latest openshift-local aws \
          --backed-url "file:///workspace" \
          --bundle-url ${BUNDLE_URL} \
          --shasum-url ${SHASUM_URL} \
          --arch ${ARCH} \
          --debug \
          --debug-level 9
```

### To upload an OpenShift Local bundle to **Azure**, run the following command, replacing the placeholder values with your specific information:

```bash
podman run --rm --name import-openshift-local -d \
    -v ${PWD}:/workspace:z \
    -e ARM_CLIENT_ID=${ARM_CLIENT_ID} \
    -e ARM_CLIENT_SECRET=${ARM_CLIENT_SECRET} \
    -e ARM_SUBSCRIPTION_ID=${ARM_SUBSCRIPTION_ID} \
    -e ARM_LOCATION_NAME=${ARM_LOCATION_NAME} \
    ghcr.io/mapt-oss/cloud-importer:latest openshift-local azure \
          --backed-url "file:///workspace" \
          --bundle-url ${BUNDLE_URL} \
          --shasum-url ${SHASUM_URL} \
          --arch ${ARCH} \
          --replicate all \
          --debug \
          --debug-level 9
```

**Parameters:**

  * `--backed-url`: The local directory, s3 bucket or an Azure blob store URL for pulumi to store the Stack files
  * `--bundle-url`: The URL of the OpenShift Local bundle you want to upload
  * `--shasum-url`: The URL of the checksum file to verify the bundle's integrity
  * `--arch`: The architecture of the bundle (e.g., `x86_64`, `arm64`).
  * `--replicate`: To replicate the image to other regions (e.g., `us-west-1,eu-east-1` or `all` to replicate to all available regions)
  * `--debug` & `--debug-level`: Optional flags to enable verbose logging for troubleshooting.

### To replicate an existing image to other regions:

```bash
podman run --rm --name import-openshift-local -d \
    -v ${PWD}:/workspace:z \
    -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \ # Use Azure credentials for Azure provider
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
    ghcr.io/mapt-oss/cloud-importer:latest replicate <aws|azure> \
          --backed-url "file:///workspace" \
          --image-id ${IMAGE_ID} \
          --region all
```

**Parameters:**

  * `--image-id`: Cloud provider specific image identifier
  * `--region`: Comma seperated list of regions or `all`

### To share an existing image with other accounts or organizations:

> [!NOTE]
> This only works for AWS at the moment

```bash
podman run --rm --name import-openshift-local -d \
    -v ${PWD}:/workspace:z \
    -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
    ghcr.io/mapt-oss/cloud-importer:latest share aws \
          --backed-url "file:///workspace" \
          --image-id ${IMAGE_ID} \
          --arch ${ARCH} \
          --account-id ${ACCOUNT_ID} \
          --debug \
          --debug-level 9
```


**Parameters:**

> [!NOTE]
> These are mutually exclusive

  * `--account-id`: Individual user account id on aws
  * `--organization-arn`: Full organization arn of an aws organization


## Troubleshooting

`cloud-importer` performs the following steps:

**1. Bundle Download:**

  * The tool first downloads the OpenShift Local bundle and its checksum from the provided URLs
      * Linux (libvirt) bundle which has the `qcow2` image is easier to convert to RAW or VHD
  * It then verifies the integrity of the downloaded bundle using the checksum
  * **Troubleshooting:** If you encounter errors at this stage, double-check the `--bundle-url` and `--shasum-url` values

**2. Disk Extraction:**

  * Extract and convert disk image to cloud provider expected format:
      * **Decompression:** The downloaded bundle (`.xz` archive with `zstd` compression) is uncompressed and files are extracted
      * **Image Location:** The tool locates the `qcow2` disk image within the extracted files
      * **Image Conversion:** AWS requires the disk image to be in `.raw` format and for Azure it should be in `.vhd` format
  * **Troubleshooting:**
      * **Corrupted Archive:** An error during decompression could indicate a corrupted download. Try removing the local bundle and running the tool again
      * **Disk Space:** Ensure it has sufficient free space to store both the downloaded bundle and the extracted disk image (~ 60GB)

**3. Upload to Cloud Provider storage (S3, blob storage):**

  * The prepared disk image is uploaded to an S3 bucket for AWS or a Storage blob for Azure, `cloud-importer` creates temporary resources for this purpose
  * **Troubleshooting:**
      * **Authentication:** Ensure your cloud provider credentials are correct and have the necessary permissions

**4. Disk Image Import:**

  * **AWS**: The tool initiates a VM import task, pointing to the uploaded disk image in S3. This process converts the disk image into an EBS snapshot
  * **Azure**: The tool creates a Compute Gallery then a Gallery Image Definition, after which an Image Version pointing to the Blob storage containing the disk image
  - **AMI/Disk Image Creation:** Once the snapshot/Galley Image Definition is created, it can be used to register a new AMI for AWS or Image Version for Azure in your account
  * **Troubleshooting:**
      * **IAM Role:** The VM import process requires a specific IAM role (e.g., `vmimport`). If this role doesn't exist or lacks the necessary permissions, `cloud-importer` will attempt to create this role for you
      * **Permissions:** Your AWS user needs permissions for EC2 VM import (`ec2:ImportSnapshot`, `ec2:DescribeImportSnapshotTasks`)

