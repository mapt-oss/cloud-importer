# cloud-importer

A tool to import and manage private VM images across cloud providers. It automates the steps required to import a disk image as a registered cloud image (AMI on AWS, Gallery Image on Azure, Custom Image on GCP) and optionally share it across accounts/projects.

## Prerequisites

Before you begin, ensure you have the following:

* **Cloud Account:** An active AWS, Azure, or GCP account
* **Cloud Credentials** (set as environment variables):

  **AWS:**
  ```bash
  AWS_ACCESS_KEY_ID
  AWS_SECRET_ACCESS_KEY
  AWS_DEFAULT_REGION
  ```

  **Azure:**
  ```bash
  ARM_CLIENT_ID
  ARM_CLIENT_SECRET
  ARM_TENANT_ID
  ARM_SUBSCRIPTION_ID
  ARM_LOCATION_NAME
  AZURE_STORAGE_ACCOUNT   # required when using azblob:// backed-url
  AZURE_STORAGE_KEY       # required when using azblob:// backed-url
  ```

  **GCP:**
  ```bash
  GOOGLE_PROJECT                  # GCP project ID where images will be created
  GOOGLE_CREDENTIALS              # Service account key JSON (inline string)
  GOOGLE_REGION                   # Default GCP region (e.g. us-central1)
  GOOGLE_IMAGE_STORAGE_LOCATIONS  # Optional: comma-separated multi-regions (default: us,eu,asia)
  ```

## Params

### Common to all commands

| Flag | Description |
|---|---|
| `--project-name` | Unique name for this import run — used to isolate Pulumi state |
| `--backed-url` | Backend for Pulumi state: `s3://bucket/path`, `azblob://container/path`, `gs://bucket/path`, or `file:///local/path` |
| `--replicate` | Replicate the image to all available regions (no-op for GCP — images are already global) |
| `--share-orgs-ids` | Comma-separated list of identifiers to share the image with: AWS org ARNs, Azure tenant IDs, or GCP project IDs |
| `--tags` | Comma-separated tags to apply: `key1=value1,key2=value2` |
| `--debug` | Enable debug logging |
| `--debug-level` | Verbosity level 1–9 (default: 3) |

### RHEL AI specific

| Flag | Description |
|---|---|
| `--image-path` | Local path to the image file (`.raw` for AWS/GCP, `.vhd` for Azure) |
| `--image-name` | Name to register the image under in the cloud provider |

### SNC (OpenShift Local) specific

| Flag | Description |
|---|---|
| `--bundle-uri` | Accessible URI to the SNC bundle (http/https/file) |
| `--shasum-uri` | Accessible URI to the bundle checksum file |
| `--arch` | Architecture: `x86_64` or `arm64` (default: `x86_64`) |

### Destroy specific

| Flag | Description |
|---|---|
| `--keep-state` | Keep Pulumi state in the backend after destroy (default: false) |
| `--force-destroy` | Remove Pulumi lock files before destroying (use to recover from a crashed import) |

### Check specific

| Flag | Description |
|---|---|
| `--image-name` | Image name to look up in the cloud provider |

---

## RHEL AI

Imports a RHEL AI disk image to a cloud provider. The raw image must be downloaded separately by an authenticated user who has agreed to the EULA. See the [RHEL AI installation guide](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux_ai/1.5/html/installing/installing_on_aws).

### AWS

```bash
podman run --rm --name import-rhelai -d \
    -v ${PWD}:/workspace:z \
    -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
    quay.io/aipcc-cicd/cloud-importer:latest rhelai aws \
        --project-name "rhelai3-136d47d1" \
        --backed-url s3://bucket/folder \
        --image-name rhelai3-136d47d1 \
        --image-path "/workspace/rhel-ai-nvidia-aws-1.5-x86_64.raw" \
        --share-orgs-ids arn:aws:organizations::XXXXX:organization/XXXXX \
        --replicate \
        --debug \
        --debug-level 9

podman logs -f import-rhelai
```

### Azure

```bash
podman run --rm --name import-rhelai-azure -d \
    -v ${PWD}:/workspace:z \
    -e ARM_TENANT_ID=${ARM_TENANT_ID} \
    -e ARM_CLIENT_ID=${ARM_CLIENT_ID} \
    -e ARM_CLIENT_SECRET=${ARM_CLIENT_SECRET} \
    -e ARM_SUBSCRIPTION_ID=${ARM_SUBSCRIPTION_ID} \
    -e ARM_LOCATION_NAME=${ARM_LOCATION_NAME} \
    -e AZURE_STORAGE_ACCOUNT=${AZURE_STORAGE_ACCOUNT} \
    -e AZURE_STORAGE_KEY=${AZURE_STORAGE_KEY} \
    quay.io/aipcc-cicd/cloud-importer:latest rhelai az \
        --project-name "rhelai3-136d47d1" \
        --backed-url azblob://blobcontainer/folder \
        --image-name rhelai3-136d47d1 \
        --image-path "/workspace/rhel-ai-nvidia-aws-1.5-x86_64.vhd" \
        --share-orgs-ids tenantId1,tenantId2 \
        --replicate \
        --debug \
        --debug-level 9

podman logs -f import-rhelai-azure
```

### GCP

```bash
podman run --rm --name import-rhelai-gcp -d \
    -v ${PWD}:/workspace:z \
    -e GOOGLE_PROJECT=${GOOGLE_PROJECT} \
    -e GOOGLE_CREDENTIALS=${GOOGLE_CREDENTIALS} \
    -e GOOGLE_REGION=${GOOGLE_REGION} \
    quay.io/aipcc-cicd/cloud-importer:latest rhelai gcp \
        --project-name "rhelai3-136d47d1" \
        --backed-url gs://bucket/folder \
        --image-name rhelai3-136d47d1 \
        --image-path "/workspace/rhel-ai-nvidia-aws-1.5-x86_64.raw" \
        --share-orgs-ids gcp-project-a,gcp-project-b \
        --debug \
        --debug-level 9

podman logs -f import-rhelai-gcp
```

> **Note:** `--replicate` is accepted but has no effect for GCP — custom images are globally available within a project once created.

---

## SNC (OpenShift Local)

Transforms the bundle generated by [snc](https://github.com/crc-org/snc), uploads it, and registers it as a cloud provider image. The resulting image can be used to create ephemeral OpenShift Local clusters.

### AWS

```bash
podman run --rm --name import-snc -d \
    -v ${PWD}:/workspace:z \
    -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
    quay.io/aipcc-cicd/cloud-importer:latest snc aws \
        --project-name "snc-4.20.0" \
        --backed-url s3://bucket/folder \
        --bundle-uri ${BUNDLE_URL} \
        --shasum-uri ${SHASUM_URL} \
        --arch ${ARCH} \
        --replicate \
        --share-orgs-ids arn:aws:organizations::XXXXX:organization/XXXXX \
        --debug \
        --debug-level 9
```

### Azure

```bash
podman run --rm --name import-snc-azure -d \
    -v ${PWD}:/workspace:z \
    -e ARM_CLIENT_ID=${ARM_CLIENT_ID} \
    -e ARM_CLIENT_SECRET=${ARM_CLIENT_SECRET} \
    -e ARM_TENANT_ID=${ARM_TENANT_ID} \
    -e ARM_SUBSCRIPTION_ID=${ARM_SUBSCRIPTION_ID} \
    -e ARM_LOCATION_NAME=${ARM_LOCATION_NAME} \
    -e AZURE_STORAGE_ACCOUNT=${AZURE_STORAGE_ACCOUNT} \
    -e AZURE_STORAGE_KEY=${AZURE_STORAGE_KEY} \
    quay.io/aipcc-cicd/cloud-importer:latest snc az \
        --project-name "snc-4.20.0" \
        --backed-url azblob://blobcontainer/folder \
        --bundle-uri ${BUNDLE_URL} \
        --shasum-uri ${SHASUM_URL} \
        --arch ${ARCH} \
        --replicate \
        --share-orgs-ids tenantId1,tenantId2 \
        --debug \
        --debug-level 9
```

### GCP

```bash
podman run --rm --name import-snc-gcp -d \
    -e GOOGLE_PROJECT=${GOOGLE_PROJECT} \
    -e GOOGLE_CREDENTIALS=${GOOGLE_CREDENTIALS} \
    -e GOOGLE_REGION=${GOOGLE_REGION} \
    quay.io/aipcc-cicd/cloud-importer:latest snc gcp \
        --project-name "snc-4.20.0" \
        --backed-url gs://bucket/folder \
        --bundle-uri ${BUNDLE_URL} \
        --shasum-uri ${SHASUM_URL} \
        --arch ${ARCH} \
        --share-orgs-ids gcp-project-a,gcp-project-b \
        --debug \
        --debug-level 9
```

---

## Check

Verifies whether an image with the given name already exists in the cloud provider. Exits `0` if found, `1` if not found, `2` on error.

### AWS

```bash
podman run --rm \
    -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
    quay.io/aipcc-cicd/cloud-importer:latest check aws \
        --image-name rhelai3-136d47d1
```

### Azure

```bash
podman run --rm \
    -e ARM_CLIENT_ID=${ARM_CLIENT_ID} \
    -e ARM_CLIENT_SECRET=${ARM_CLIENT_SECRET} \
    -e ARM_TENANT_ID=${ARM_TENANT_ID} \
    -e ARM_SUBSCRIPTION_ID=${ARM_SUBSCRIPTION_ID} \
    -e ARM_LOCATION_NAME=${ARM_LOCATION_NAME} \
    quay.io/aipcc-cicd/cloud-importer:latest check az \
        --image-name rhelai3-136d47d1
```

### GCP

```bash
podman run --rm \
    -e GOOGLE_PROJECT=${GOOGLE_PROJECT} \
    -e GOOGLE_CREDENTIALS=${GOOGLE_CREDENTIALS} \
    quay.io/aipcc-cicd/cloud-importer:latest check gcp \
        --image-name rhelai3-136d47d1
```

---

## Destroy

Destroys all cloud resources associated with an import run and removes the Pulumi state. Run with the same `--project-name` and `--backed-url` used during import. Credentials must match the provider used for the original import.

### AWS

```bash
podman run --rm \
    -e AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
    -e AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
    -e AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION} \
    quay.io/aipcc-cicd/cloud-importer:latest destroy \
        --project-name "snc-4.20.0" \
        --backed-url s3://bucket/folder
```

### Azure

```bash
podman run --rm \
    -e ARM_CLIENT_ID=${ARM_CLIENT_ID} \
    -e ARM_CLIENT_SECRET=${ARM_CLIENT_SECRET} \
    -e ARM_TENANT_ID=${ARM_TENANT_ID} \
    -e ARM_SUBSCRIPTION_ID=${ARM_SUBSCRIPTION_ID} \
    -e ARM_LOCATION_NAME=${ARM_LOCATION_NAME} \
    quay.io/aipcc-cicd/cloud-importer:latest destroy \
        --project-name "snc-4.20.0" \
        --backed-url azblob://blobcontainer/folder
```

### GCP

```bash
podman run --rm \
    -e GOOGLE_PROJECT=${GOOGLE_PROJECT} \
    -e GOOGLE_CREDENTIALS=${GOOGLE_CREDENTIALS} \
    quay.io/aipcc-cicd/cloud-importer:latest destroy \
        --project-name "snc-4.20.0" \
        --backed-url gs://bucket/folder
```

---

## Developer Testing

For local testing, store Pulumi state in the mounted workspace directory — no cloud storage bucket needed. Load credentials from Bitwarden and pass them with name-only `-e` flags so values never appear in shell history or `ps` output.

### Bitwarden item conventions

| Bitwarden item | `username` field | `password` field | `notes` field |
|---|---|---|---|
| `AWS_ACCESS` | `AWS_ACCESS_KEY_ID` value | `AWS_SECRET_ACCESS_KEY` value | — |
| `AZ_SP` | `ARM_CLIENT_ID` value | `ARM_CLIENT_SECRET` value | — |
| `AZ_STORAGE` | `AZURE_STORAGE_ACCOUNT` value | `AZURE_STORAGE_KEY` value | — |
| `GCP_SA_KEY` | — | — | service account key JSON |

### Load credentials into your shell

First, unlock your Bitwarden vault and establish a session:

```bash
export BW_SESSION=$(bw unlock --raw)
```

**AWS:**
```bash
export AWS_ACCESS_KEY_ID=$(bw get username "AWS_ACCESS")
export AWS_SECRET_ACCESS_KEY=$(bw get password "AWS_ACCESS")
export AWS_DEFAULT_REGION=us-east-1
```

**Azure:**
```bash
export ARM_CLIENT_ID=$(bw get username "AZ_SP")
export ARM_CLIENT_SECRET=$(bw get password "AZ_SP")
export ARM_TENANT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export ARM_SUBSCRIPTION_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
export ARM_LOCATION_NAME=eastus
export AZURE_STORAGE_ACCOUNT=$(bw get username "AZ_STORAGE")
export AZURE_STORAGE_KEY=$(bw get password "AZ_STORAGE")
```

**GCP:**
```bash
export GOOGLE_PROJECT=my-gcp-project-id
export GOOGLE_REGION=us-central1
export GOOGLE_CREDENTIALS=$(bw get notes "GCP_SA_KEY" | jq -c .)  # compact multiline JSON to single line
```

### Run with local Pulumi state

```bash
podman run --rm --name import-rhelai -d \
    --user 0 \
    -v ${PWD}:/workspace:z \
    -e AWS_ACCESS_KEY_ID \
    -e AWS_SECRET_ACCESS_KEY \
    -e AWS_DEFAULT_REGION \
    quay.io/aipcc-cicd/cloud-importer:latest rhelai aws \
        --project-name "rhelai-dev-test" \
        --backed-url "file:///workspace" \
        --image-name "rhelai-dev-test" \
        --image-path "/workspace/rhel-ai-nvidia-aws-1.5-x86_64.raw" \
        --debug \
        --debug-level 9

podman logs -f import-rhelai
```

Replace `aws` with `az` or `gcp` and swap the `-e` flags to match the provider. Pulumi state is written to `${PWD}/rhelai-dev-test/` — delete it when done.

---

## Testing VMs

After a successful import, launch a short-lived test VM to confirm the image boots correctly and is the expected OS/version. Remember to delete the test VM when done.

### AWS

```bash
# Launch a test instance
aws ec2 run-instances \
    --image-id <ami-id-from-import-output> \
    --instance-type t3.medium \
    --region ${AWS_DEFAULT_REGION} \
    --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=image-test}]' \
    --query 'Instances[0].InstanceId' --output text

# Wait for it to be running, then SSH
aws ec2 wait instance-running --instance-ids <instance-id>
ssh ec2-user@<public-ip>

# Verify OS / RHEL AI version
cat /etc/os-release
ilab --version   # for RHEL AI images

# Clean up
aws ec2 terminate-instances --instance-ids <instance-id>
```

### Azure

```bash
# Launch a test VM from the gallery image
az vm create \
    --resource-group aipcc-productization \
    --name image-test \
    --image aipcc-productization/aipcc-gallery/rhelai3-136d47d1/latest \
    --size Standard_D4s_v3 \
    --admin-username azureuser \
    --generate-ssh-keys

# SSH and verify
ssh azureuser@<public-ip>
cat /etc/os-release
ilab --version   # for RHEL AI images

# Clean up
az vm delete --resource-group aipcc-productization --name image-test --yes
```

### GCP

```bash
# Launch a test VM (use --preemptible for a cheaper spot-equivalent test)
gcloud compute instances create image-test \
    --image rhelai3-136d47d1 \
    --image-project ${GOOGLE_PROJECT} \
    --machine-type n2-standard-4 \
    --zone ${GOOGLE_REGION}-a \
    --preemptible

# SSH and verify
gcloud compute ssh image-test --zone ${GOOGLE_REGION}-a
cat /etc/os-release
ilab --version   # for RHEL AI images

# Clean up
gcloud compute instances delete image-test --zone ${GOOGLE_REGION}-a --quiet
```

---

## Release

Versioned images are published to `quay.io/aipcc-cicd/cloud-importer` on every tag push.

To trigger a release:

1. Ensure your changes are merged to `main` or a `release-*` branch
2. Push a tag matching `v*.*.*` (e.g. `v1.0.0`)
3. The release workflow will:
   - Copy the image from ghcr to `quay.io/aipcc-cicd/cloud-importer:v1.0.0`
   - Also tag it as `quay.io/aipcc-cicd/cloud-importer:latest`
   - Create a GitHub Release with auto-generated notes

Tags must point to a commit on `main` or a `release-*` branch, otherwise the workflow will fail.

---

## Troubleshooting

`cloud-importer` performs the following steps:

**1. Bundle Download** *(SNC only)*

* Downloads the OpenShift Local bundle and its checksum from the provided URIs
  * The Linux (libvirt) bundle containing the `qcow2` image is easiest to convert to raw/VHD/tar.gz
* Verifies the bundle integrity using the checksum
* **Troubleshooting:** Double-check `--bundle-uri` and `--shasum-uri` values if errors occur here

**2. Disk Extraction** *(SNC only)*

* Decompresses the `.xz` archive and extracts files
* Locates the `qcow2` disk image and converts it to the provider's required format:
  * **AWS:** `.raw`
  * **Azure:** `.vhd`
  * **GCP:** `disk.raw.tar.gz` (a compressed tar archive containing `disk.raw`)
* **Troubleshooting:**
  * Corrupted archive: remove the local bundle and re-run
  * Disk space: ensure ~60 GB free for the downloaded bundle and extracted image

**3. Upload to cloud storage**

* Uploads the prepared disk image to temporary cloud storage (S3, Azure Blob, or GCS)
* **Troubleshooting:** Verify credentials have write permissions to the storage service

**4. Image registration**

* **AWS:** Initiates a VM import task → EBS snapshot → AMI registration
* **Azure:** Creates a Compute Gallery, Gallery Image Definition, and Image Version pointing to the blob
* **GCP:** Creates a Compute Engine Custom Image from the GCS source URI
* **Troubleshooting:**
  * **AWS IAM role:** The `vmimport` role is created automatically if it doesn't exist. If import fails, verify your user has `ec2:ImportSnapshot` and `ec2:DescribeImportSnapshotTasks` permissions
  * **GCP:** Ensure the `compute.images.create` permission is granted to the service account whose credentials are in `GOOGLE_CREDENTIALS`

**5. Stuck imports / lock files**

If a previous run crashed and left a Pulumi lock, re-run with `--force-destroy` added to the destroy command to clear the lock before retrying.
