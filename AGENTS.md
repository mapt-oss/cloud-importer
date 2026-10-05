# cloud-importer — Agent Context

## What this project does

CLI tool (`cloud-importer`) that imports disk images (RHEL AI, SNC/OpenShift Local) to cloud provider custom image registries. Uses **Pulumi inline programs** (Go) to manage cloud resources declaratively.

Supported providers: AWS, Azure, GCP, IBM Cloud.

## Architecture

Two-stack pattern per import:

1. **Ephemeral stack** — creates temporary storage (S3/blob/COS bucket), uploads the disk image, exports metadata as stack outputs, tears itself down after the persistent stack is created.
2. **Persistent (register) stack** — reads ephemeral outputs and registers the cloud image (AMI, managed image, GCE image, IBM VPC custom image). Survives destroy of ephemeral.

### Key packages

| Path | Purpose |
|---|---|
| `cmd/importer/cmd/` | Cobra CLI commands (`rhelai`, `snc`, `destroy`, `check`) |
| `pkg/manager/` | Orchestration: `RHELAI()`, `SNC()`, `Destoy()` functions, stack lifecycle |
| `pkg/manager/context/` | Singleton context: backed URL translation, project name, flags |
| `pkg/manager/providers.go` | Routes backed URL scheme → provider (`s3://`→AWS, `cos://`→IBM, etc.) |
| `pkg/provider/{aws,azure,gcp,ibm}/` | Provider implementations: ephemeral upload, image registration, credentials |
| `pkg/util/bundle/` | SNC bundle download, shasum check, extraction (`extract.sh`) |

### `--backed-url` schemes

| Scheme | Provider | Notes |
|---|---|---|
| `s3://bucket/path` | AWS | Standard AWS S3 |
| `azblob://container/path` | Azure | Requires `AZURE_STORAGE_ACCOUNT` + `AZURE_STORAGE_KEY` |
| `gs://bucket/path` | GCP | |
| `cos://bucket/path` | IBM | Translated to `s3://...?endpoint=IBM_COS_HOST&s3ForcePathStyle=true` at init; HMAC keys auto-mapped to `AWS_ACCESS_KEY_ID`/`SECRET` |
| `file:///path` | Any | Local state, for development |

Project name is appended to the backed URL **before** any query string:
`cos://bucket/path` → `s3://bucket/path/{project-name}?endpoint=...`

Convention: `cos://aipcc-productization/cloud-importer`

### Provider routing

`pkg/manager/context/context.go` stores `rawScheme` (original URL scheme before translation).
`pkg/manager/providers.go` uses `context.RawScheme() == "cos"` to route translated `s3://` IBM COS URLs to the IBM provider (whose `DeleteLocks`/`CleanupState` are intentional no-ops).

## IBM Cloud specifics

### Environment variables

| Variable | Description | Fallback |
|---|---|---|
| `IBMCLOUD_API_KEY` | IBM Cloud API key | — |
| `IBMCLOUD_REGION` | Region (e.g. `us-south`) | `IC_REGION` |
| `IBMCLOUD_COS_ACCESS_KEY` | HMAC access key (upload + Pulumi state) | — |
| `IBMCLOUD_COS_SECRET_KEY` | HMAC secret key (upload + Pulumi state) | — |
| `IBMCLOUD_COS_ENDPOINT` | Override COS endpoint (private/custom) | — |

`IC_*` prefixed vars are legacy aliases — always prefer `IBMCLOUD_*`.

### Catalog publish flow

After the VPC image is registered, `PublishVPCImage` (in `publish.go`) creates or reuses a private catalog offering and drives the version through the lifecycle:

1. Import the version with full VPC image metadata (OS family, architecture, file size) — fetched live from the VPC API.
2. Call `PrereleaseVersion` directly (`new → prerelease`). IBM only requires Schematics validation for public IBM Cloud catalog publish, not for private catalogs.
3. Attempt `ConsumableVersion`; if IBM rejects it with "not consumable" (because the version is unvalidated), stay in `prerelease` — this is sufficient for sharing with specific accounts via the offering access list.
4. Reconcile the per-offering account access list (`--share-orgs-ids`).

The offering name is derived from the **image name** (e.g. `openshift-local-4-22-12-x86-64`), not the project name, so it matches the VPC image name convention.

### Image formats

- **RHEL AI**: accepts `.qcow2` directly (IBM VPC requires qcow2, RHEL AI provides it natively).
- **SNC**: `extract.sh` converts `bundle/crc.qcow2 → disk.qcow2` directly (no raw intermediate), skipping the large intermediate `disk.raw` that would OOM on big images. The qcow2 is then uploaded to COS and registered as a VPC custom image.

### Pulumi state on IBM COS

Pulumi's S3 backend is used with IBM COS endpoint override. `context.Init()` handles the full translation:
- `cos://bucket/path` → `s3://bucket/path?endpoint=s3.REGION.cloud-object-storage.appdomain.cloud&s3ForcePathStyle=true`
- `IBMCLOUD_COS_ACCESS_KEY` → `AWS_ACCESS_KEY_ID` (if not already set)
- `IBMCLOUD_COS_SECRET_KEY` → `AWS_SECRET_ACCESS_KEY` (if not already set)

## SNC bundle processing

`pkg/util/bundle/extract.sh` handles:
1. Download `.crcbundle` + verify sha256
2. Decompress (zstd → tar)
3. Provider-specific conversion:
   - **IBM**: `qemu-img convert bundle/crc.qcow2 disk.qcow2` — direct, no raw intermediate (avoids OOM on large images)
   - **All others**: `qemu-img convert bundle/crc.qcow2 disk.raw` first, then:
     - Azure: `disk.raw → disk.vhd`, removes raw
     - AWS/GCP: keeps `disk.raw`

Bundle and shasum URIs accept `https://`, `http://`, `file://`, or local paths.

## Build

```bash
# Binary only
make build

# Container image (amd64)
make oci-build-amd64 IMG=cloud-importer:dev
# or directly:
podman build --platform linux/amd64 -t cloud-importer:dev -f oci/Containerfile .
```

Container is based on `quay.io/almalinuxorg/9-base`. Includes: `qemu-img`, `aws-cli`, `azcopy`, `zstd`, Pulumi CLI, and all Pulumi provider plugins.

## CLI structure

```
cloud-importer
├── rhelai {aws,az,gcp,ibm}   --image-path --image-name --backed-url --project-name
├── snc    {aws,az,gcp,ibm}   --bundle-uri --shasum-uri --arch --backed-url --project-name
├── destroy                    --backed-url --project-name [--keep-state] [--force-destroy]
└── check  {aws,az,gcp,ibm}   --image-name
```

`destroy` has no provider subcommand — provider is inferred from `--backed-url` scheme.

## Open issues / related

- [mapt#912](https://github.com/redhat-developer/mapt/issues/912) — apply same `cos://` backed-url translation to mapt
- Upstream PR: [cloud-importer#105](https://github.com/mapt-oss/cloud-importer/pull/105) — IBM Cloud VPC support
