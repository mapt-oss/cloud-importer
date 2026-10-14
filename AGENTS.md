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

### Replication (`--replicate`)

IBM VPC requires a COS bucket accessible from the target region. Rather than uploading the disk once per region, the code uses IBM COS **cross-region geo buckets** to minimise copy work.

**`vpcRegionGeo` map** (`cos.go`):
- `us-south`, `us-east` → `us` geo (`s3.us.cloud-object-storage.appdomain.cloud`, constraint `us-standard`)
- `eu-gb`, `eu-de`, `eu-es` → `eu` geo (`s3.eu.cloud-object-storage.appdomain.cloud`, constraint `eu-standard`)
- `jp-tok`, `jp-osa`, `au-syd` → `ap` geo (`s3.ap.cloud-object-storage.appdomain.cloud`, constraint `ap-standard`)
- `ca-tor`, `ca-mon`, `br-sao`, `in-che`, `in-mum` → no geo (individual regional buckets, constraint `<region>-standard`)

**`ibmCOSSupportedRegions`** (13 regions) — all regions with IBM COS endpoints (used by `otherVPCRegions` to filter the live VPC regions list from the API).

**`replicaGroup` struct** (`image.go`) — one group per geo or per no-geo region:
- `needsCopy=false`: same geo as primary, reuses primary bucket and `cosURI`, VPC images start immediately.
- `needsCopy=true`: different geo or no-geo, creates a new bucket then streams the disk once from the primary bucket (no temp file — piped directly between S3 endpoints).

**`buildReplicaGroups`** groups replica VPC regions by `ibmVPCRegionGeo`:
- Same-geo regions (e.g. `us-east` when primary is `us-south`) → `needsCopy=false` group.
- Each other geo → one `needsCopy=true` group covering all VPC regions in that geo.
- No-geo regions → one `needsCopy=true` group per region.

**`replicateGroup`** (per group):
1. If `needsCopy`: create COS bucket (`s3api create-bucket --create-bucket-configuration LocationConstraint=<constraint>`), then stream disk with retry loop.
2. Wait for bucket propagation: 20 × 15s `head-bucket` poll before upload (IBM COS cross-region buckets have eventual consistency on creation).
3. Create VPC `IsImage` for every region in the group, all pointing at the same `cos://geo/bucket/disk.qcow2` URI. IBM VPC confirmed to accept cross-region geo URIs.

**Upload optimisations** (streaming copy in `replicateGroup`):
- `AWS_S3_MULTIPART_CHUNKSIZE=128MB` — reduces multipart requests from ~5000 to ~312 for a 40 GB image.
- `AWS_MAX_CONCURRENT_REQUESTS=32` — maximises TCP window utilisation on cross-region links.
- Streaming pipe: `aws s3 cp src - | aws s3 cp - dst --expected-size $size` — no temp file, no disk usage.
- Post-transfer size verification: compares `src_size` and `dst_size` via `head-object`; fails the loop on mismatch.
- 3-attempt retry loop with 60s pause between attempts.

**Bucket creation idempotency**: `create-bucket || head-bucket` — if the bucket already exists (e.g. from a previous run with a different LocationConstraint), the fallback `head-bucket` confirms access and the command succeeds.

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
