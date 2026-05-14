# GCP Provider — Feature Spec

**Project:** cloud-importer  
**Ticket:** AIPCC-14898  
**Feature:** Add Google Cloud Platform (GCP) as a third cloud provider  
**Status:** Approved for implementation  

---

## Open Questions

Items that remain open or should be verified before/during implementation:

| # | Question | Status |
|---|---|---|
| 1 | The GCP console shows `aipcc-cicd` has no parent organization. **Confirm:** (a) does the logged-in user have sufficient privileges to see org membership if it existed? (b) is there a plan to place `aipcc-cicd` (or other Red Hat GCP projects) under a GCP organization in future? This affects whether org-level image sharing ever becomes an option. | Needs confirmation from GCP admin |
| 2 | When we share an image with a target project, we grant `roles/compute.imageUser` to a specific identity in that project. For Spot VMs (which GCP manages), the right target is the project's **Compute Engine service agent** (`service-{PROJECT_NUMBER}@compute-system.iam.gserviceaccount.com`). Confirm this is how consuming teams launch VMs — i.e. they are not using a custom service account to create VMs, which would require granting the role to that SA instead. | Ask GCP admin / consuming team |
| 3 | Image storage location default: store in all three GCP multi-regions (`us`, `eu`, `asia`) so Spot VMs scheduled anywhere globally get fast boot times. Override via `GOOGLE_IMAGE_STORAGE_LOCATIONS` env var for callers who only need one geography. | See Decision 5 — leaning toward this default |
| 4 | **Credentials for testing:** What service account or credentials should a developer use to test GCP imports against the `aipcc-cicd` project? Who provisions them, and what is the process to request access? | Ask project lead / GCP admin |
| 5 | **Pulumi state file location conventions:** Every import run requires `--backed-url` pointing to a storage location for the Pulumi state file. Is there an established convention for where these live (e.g. a shared S3 bucket, a dedicated GCS bucket per provider)? Without a centralized or well-known location there is no easy way to get a complete picture of all images currently under management — a stale or lost state file also means `destroy` cannot clean up resources. Should the team define a standard backend location per provider, and/or maintain a separate image inventory? | Ask project lead — no convention observed in the codebase today |

---

## Overview

cloud-importer automates importing and managing private VM images across cloud providers. It currently supports AWS (via EC2 AMIs) and Azure (via Compute Gallery). This spec adds GCP as a third provider using Compute Engine Custom Images and Cloud Storage (see [Appendix: GCP Image Storage Options](#appendix-gcp-image-storage-options) for why Custom Images were chosen over alternatives).

All existing operations will be supported: `rhelai gcp`, `snc gcp`, `check gcp`, and `destroy` with a `gs://` Pulumi state backend.

---

## GCP Concepts (for readers familiar with AWS)

| AWS Term | GCP Equivalent | Key Difference |
|---|---|---|
| S3 Bucket | Cloud Storage bucket (GCS) | Same concept, different API |
| AMI | Compute Engine Custom Image | **GCP images are globally available** — no per-region copy needed |
| IAM Role (VM Import) | Not needed | GCP creates images directly from a GCS URI |
| Org ARN (for sharing) | GCP Project ID | Grant `roles/compute.imageUser` to the target project's compute service agent |
| `s3://` backend | `gs://` backend | Pulumi state stored in GCS |

**The most important difference:** AWS AMIs are regional — to use one in `us-west-2` you must copy it from `us-east-1`. GCP images are **global within a project** — once created, they are automatically available in every region. This simplifies the persistent stack significantly.

---

## Image Format

GCP's Compute Engine `images.create` API requires the source in Cloud Storage to be a `.tar.gz` archive containing a file named `disk.raw`. The ephemeral stack will add a compression step:

```
disk.raw  →  tar czf disk.raw.tar.gz  →  upload to GCS  →  compute.Image
```

For SNC imports, bundle extraction already produces a raw disk file (same as AWS). The only addition is the tar.gz compression before upload.

---

## Design Decisions

### 1. `--replicate` behavior for GCP

**Question:** The `--replicate` flag tells the tool to copy the image to all available regions. Since GCP images are already global, what should this flag do?

**Alternatives considered:**

| Option | Description | Drawback |
|---|---|---|
| **No-op with info log** ✓ | Accept the flag, log that images are already global, do nothing | None — keeps CLI consistent |
| Omit flag from GCP subcommands | Don't expose `--replicate` for `gcp` at all | Creates an inconsistent CLI surface; scripts that pass `--replicate` to all providers would break |
| Repurpose as "share with all org projects" | Make it trigger org-wide IAM grants | Conflates two separate concerns; semantics differ too much from AWS/Azure |

**Decision: No-op with info log.** Keeps the CLI surface consistent across all three providers. Callers can safely pass `--replicate` to any provider without branching.

---

### 2. `--share-orgs-ids` targets for GCP

**Question:** In AWS this flag takes organization ARNs; in Azure, tenant IDs. What identifier should GCP use?

**Alternatives considered:**

| Option | Example Value | Drawback |
|---|---|---|
| **GCP Project IDs** ✓ | `my-project-123` | None — natural GCP unit |
| GCP Organization IDs | `organizations/123456789` | Grants access to all projects in the org, which is broader than intended |
| Full IAM member strings | `serviceAccount:sa@proj.iam.gserviceaccount.com` | Verbose; callers must know the full principal format, unlike the simpler IDs used for AWS/Azure |

**Decision: GCP Project IDs.** This is the natural GCP equivalent of an AWS account or Azure subscription — the billing and IAM boundary. Implementation grants `roles/compute.imageUser` on the image to each target project's Compute Engine service agent (`service-{PROJECT_NUMBER}@compute-system.iam.gserviceaccount.com`), looked up via the Resource Manager API.

---

### 3. Pulumi state backend

**Question:** Should GCS (`gs://bucket/path`) be supported as a Pulumi state backend, alongside the existing `s3://` (AWS) and `azblob://` (Azure) options?

**Alternatives considered:**

| Option | Description | Drawback |
|---|---|---|
| **Yes, add `gs://` support** ✓ | Add GCS lock/state cleanup code mirroring `s3.go` | Small amount of new code |
| No — reuse `s3://` or `file://` | Users store GCP import state in AWS S3 or locally | Awkward cross-provider dependency; feels inconsistent |

**Decision: Yes.** Keeping state in GCS when running GCP workloads is the natural, self-contained choice. A new `gcs.go` file will mirror the existing `pkg/provider/aws/s3.go` for lock deletion and state cleanup.

---

### 4. Credentials environment variables

**Question:** GCP supports multiple authentication approaches. Which should the tool use?

**Alternatives considered:**

| Option | Env Vars | Notes |
|---|---|---|
| **`GOOGLE_PROJECT` + `GOOGLE_CREDENTIALS`** ✓ | Project ID + inline JSON string | No file path dependency; consistent with how AWS uses inline key/secret strings |
| `GOOGLE_PROJECT` + `GOOGLE_APPLICATION_CREDENTIALS` | Project ID + path to JSON file | Standard gcloud SDK default, but requires a file to exist at a specific path — fragile in containers |
| Support both | Either of the above | Most flexible, but more credential logic to maintain |

**Decision: `GOOGLE_PROJECT` + `GOOGLE_CREDENTIALS` (inline JSON).** Aligns with the existing pattern — AWS uses `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` (inline strings), not file paths. Containers can inject credentials as environment variables without needing mounted files.

---

### 5. Image storage location

**Background:** GCP Compute Engine images have a `storageLocations` field that controls where the image data is physically stored (in Cloud Storage). This is separate from image *availability* — images are always accessible from any region within a project. Storage location affects how far GCP has to transfer data when creating a boot disk for a new VM. For large images (10–50 GB), cross-region transfer can add several minutes to first-launch time.

**Context:** The primary consumers of these images are **Spot VMs** — GCP's interruptible, lower-cost VM type that GCP schedules wherever spare capacity exists globally. Because Spot VMs can land in any region worldwide, having the image stored close to every major geography is important for consistent boot times.

**Alternatives considered:**

| Option | `storageLocations` value | Trade-off |
|---|---|---|
| **All multi-regions** ✓ | `["us", "eu", "asia"]` | Best global performance; slightly higher storage cost |
| US only | `["us"]` | Fast in US; slower first launch in EU/Asia |
| Leave unset | `[]` (GCP default) | GCP picks nearest to image source; caches on first use — unpredictable latency for Spot VMs |

**Decision: Default to `["us", "eu", "asia"]`.** Spot VMs can be scheduled anywhere; storing the image in all three multi-regions ensures consistently fast boot disk creation globally regardless of where GCP schedules the VM.

**Override mechanism:** No existing CLI flag maps to this concept. Rather than adding a new flag for a deployment-level setting, expose it as an environment variable:

```bash
GOOGLE_IMAGE_STORAGE_LOCATIONS=us,eu,asia   # default
GOOGLE_IMAGE_STORAGE_LOCATIONS=us           # override for US-only deployments
GOOGLE_IMAGE_STORAGE_LOCATIONS=eu           # override for EU-only deployments
```

This follows the existing pattern where credentials and region are env vars, while per-image parameters (image name, replicate, share targets) are CLI flags.

---

## What Gets Built

### New files

```
pkg/provider/gcp/
├── gcp.go       # Provider struct, credential mapping, sourceHostingPlace()
├── rhelai.go    # RHELAIEphemeral() — compress raw→tar.gz, upload to GCS
├── snc.go       # SNCEphemeral() — extract bundle, compress, upload to GCS
├── image.go     # ImageRegister() — compute.Image from GCS + IAM sharing
├── check.go     # ImageExists() — lookup image by name via Compute API
├── util.go      # GCS bucket, upload/compress helpers (local.Command wrappers)
└── gcs.go       # DeleteLocks(), CleanupState() for gs:// backend
```

### Modified files

| File | Change |
|---|---|
| `pkg/manager/providers.go` | Add `GCP Provider = "gcp"` constant and factory case |
| `pkg/manager/manager.go` | Add `gs://` branch in `deleteLocks()` |
| `cmd/importer/cmd/rhelai.go` | Add `gcp` subcommand |
| `cmd/importer/cmd/snc.go` | Add `gcp` subcommand |
| `cmd/importer/cmd/check.go` | Add `gcp` subcommand |
| `go.mod` | Add `pulumi-gcp/sdk/v8`, `cloud.google.com/go/compute/apiv1`, `cloud.google.com/go/storage` |

---

## Environment Variables

```bash
GOOGLE_PROJECT                  # GCP project ID where images will be created
GOOGLE_CREDENTIALS              # Service account key JSON (inline string, not file path)
GOOGLE_REGION                   # Default GCP region (e.g. us-central1)
GOOGLE_IMAGE_STORAGE_LOCATIONS  # Comma-separated multi-regions for image storage (default: us,eu,asia)
                                # Tool-invented; follows GOOGLE_ prefix convention (same as ARM_LOCATION_NAME for Azure)
```

---

## Example Usage

```bash
# Import RHEL AI image to GCP
cloud-importer rhelai gcp \
  --project-name "rhelai-prod-123" \
  --backed-url "gs://my-state-bucket/cloud-importer-state" \
  --image-path "/workspace/image.raw" \
  --image-name "rhelai-prod" \
  --share-orgs-ids "partner-project-a,partner-project-b"

# Import SNC bundle to GCP
cloud-importer snc gcp \
  --project-name "snc-prod-123" \
  --backed-url "gs://my-state-bucket/cloud-importer-state" \
  --bundle-uri "https://example.com/bundle.tar.xz" \
  --shasum-uri "https://example.com/bundle.tar.xz.sha256" \
  --arch x86_64

# Check if image exists
cloud-importer check gcp --image-name "rhelai-prod"

# Destroy
cloud-importer destroy \
  --project-name "rhelai-prod-123" \
  --backed-url "gs://my-state-bucket/cloud-importer-state"
```

---

## Appendix: GCP Image Storage Options

Several GCP services can store disk images. This section documents the alternatives considered and why Compute Engine Custom Images were chosen.

### 1. Compute Engine Custom Images ✓ (chosen)

Exactly what AWS AMIs and Azure Gallery Images are — a registered VM disk image in GCP's image catalog.

**Pros:**
- Direct equivalent to AMIs — same mental model
- Single API call to create from a GCS URI
- Global availability within a project automatically
- Versioned, taggable (via labels), shareable via IAM
- First-class `gcloud compute instances create --image` support

**Cons:**
- Stored at the project level, not in a dedicated registry
- No built-in versioning beyond image families (though naming conventions handle this)

---

### 2. Artifact Registry

Artifact Registry stores containers, Maven/npm packages, and generic artifacts. There is no VM image type.

**Verdict: Not applicable.** It does not support bootable disk images.

---

### 3. Cloud Storage only (no image registration)

Store the `disk.raw.tar.gz` in GCS and skip the `compute.Image` registration step entirely.

**Pros:**
- Simpler — no registration step
- Cheaper storage than registered images

**Cons:**
- Cannot launch a VM directly from a GCS object — requires an extra import step every time
- No IAM-level image sharing
- Defeats the purpose of importing — a registered image is the actual deliverable

**Verdict: Not viable** for a tool whose goal is a launchable, shareable VM image.

---

### 4. Cloud Storage + VM Image Import via Cloud Build

GCP's `gcloud compute images import` service uses Cloud Build under the hood to handle format conversion (VMDK, VHD, OVA, raw → GCP-compatible).

**Pros:**
- Handles more input formats automatically
- Useful when the source image is not already a raw disk

**Cons:**
- Requires Cloud Build API to be enabled in the project
- Significantly slower — spins up a Cloud Build job, typically 20–40 minutes
- More moving parts; harder to debug failures
- Deprecated by Google in favor of direct image creation from GCS for raw images

**Verdict: Overkill.** Since the tool controls the image pipeline and can produce `disk.raw.tar.gz` directly, the simpler direct-from-GCS approach is faster and has fewer dependencies.

---

### 5. Compute Engine Machine Images

A Machine Image captures an entire running VM instance — disk, metadata, network config, etc. It is a snapshot of a live VM, not an imported disk image.

**Pros:**
- Captures full VM state including attached disks and configuration

**Cons:**
- Requires a running VM to capture from — cannot be created from a raw disk file
- Wrong tool for the import use case entirely

**Verdict: Not applicable.**

---

### Summary

| Option | Viable? | Reason |
|---|---|---|
| **Compute Engine Custom Images** | **Yes ✓** | AMI equivalent — direct GCS import, global, IAM-shareable |
| Artifact Registry | No | Does not support VM images |
| GCS only (no registration) | No | Cannot launch VMs directly from GCS objects |
| Cloud Build image import | Possible but worse | Slow, extra dependencies, deprecated for raw images |
| Machine Images | No | Requires a live VM; wrong use case |

---

## Appendix: GCP Image Sharing — Mechanics and Design Decision

This appendix explains how GCP image sharing works, what sharing models exist, and why project IDs were chosen as the identifier for `--share-orgs-ids`.

### Background: GCP Resource Hierarchy

GCP organizes resources in a three-level hierarchy:

```
Organization  (one per company domain, e.g. redhat.com — optional)
    └── Folders  (optional grouping: teams, products, business units)
          └── Projects  (the fundamental unit of billing, IAM, and APIs)
                └── Resources  (VMs, images, buckets, etc.)
```

A key point: **projects can exist without a parent organization**. This is common for teams that bootstrapped GCP access independently, used a billing account rather than Google Workspace, or predated a company's org governance rollout. The `aipcc-cicd` project is currently in this state — no parent organization.

Custom Compute Engine images are **project-scoped by default**: once created in a project, the image is available in all regions within that project but is not visible to any other project unless explicitly shared via IAM.

---

### How Sharing Works

Sharing a GCP image means granting the `roles/compute.imageUser` IAM role on the image resource to a principal (a user, service account, group, or broader entity). The grantee can then reference the image when creating VMs, even from a different project.

The IAM binding goes on the *image itself* (not the project), which means sharing is per-image and does not expose anything else in the source project.

```bash
# Example: share an image with a specific service account in another project
gcloud compute images add-iam-policy-binding IMAGE_NAME \
  --project=SOURCE_PROJECT \
  --member='serviceAccount:sa@target-project.iam.gserviceaccount.com' \
  --role='roles/compute.imageUser'
```

---

### Sharing Models Considered

#### Model 1: GCP Project IDs ✓ (chosen)

Pass a list of GCP project IDs (e.g. `partner-project-a,partner-project-b`). For each, grant `roles/compute.imageUser` on the image to that project's **Compute Engine service agent** — the service account GCP automatically creates for each project to run compute workloads:

```
serviceAccount:service-{PROJECT_NUMBER}@compute-system.iam.gserviceaccount.com
```

The project number is looked up from the project ID via the Resource Manager API.

**Pros:**
- Fine-grained — share only with the projects that need access
- Direct equivalent to listing individual AWS account IDs
- Works without a GCP organization

**Cons:**
- Callers must know the project IDs of their consumers
- More verbose than org-level sharing when many projects need access

**Why chosen:** `aipcc-cicd` has no parent organization, making org-level sharing impossible. Project IDs are the natural next level down and the only viable mechanism given the current GCP setup.

---

#### Model 2: GCP Organization ID

Pass a GCP organization ID (e.g. `123456789012`). Grant `roles/compute.imageUser` at the organization resource, making the image accessible to all projects under that org.

The IAM member would be the org resource itself, and the binding would be set on the org node rather than the image:

```bash
gcloud organizations add-iam-policy-binding ORG_ID \
  --member='serviceAccount:...' \
  --role='roles/compute.imageUser'
```

**Pros:**
- Single identifier covers all projects in the org — analogous to an AWS org ARN
- Simpler for callers when all consumers are in the same org

**Cons:**
- **Requires a GCP organization to exist** — `aipcc-cicd` currently has no parent org, so this model is not available
- Broader than necessary — grants access to every project in the org, not just the ones that need it
- Requires org-level IAM admin permissions to set, which may not be available

**Why not chosen:** No GCP organization exists for the `aipcc-cicd` project. If Red Hat later moves this project under a GCP org, this model could be revisited.

---

#### Model 3: GCP Folder ID

Pass a GCP folder ID. Grant access to all projects within a specific folder (e.g., all RHOAI-related projects grouped under an `ai-products/` folder).

**Pros:**
- More targeted than org-level, less manual than project-by-project

**Cons:**
- Also requires an organization (folders only exist within orgs)
- Callers must know folder IDs, which are less intuitive than project IDs
- Folder IAM propagation rules can be complex

**Why not chosen:** Same blocker as org-level — no organization exists. Also adds complexity without clear benefit over project IDs.

---

#### Model 4: Domain (`domain:redhat.com`)

GCP IAM supports a `domain:` principal type that grants access to all users authenticated with a specific Google Workspace or Cloud Identity domain.

```bash
--member='domain:redhat.com'
```

**Pros:**
- Simple — one string covers all `@redhat.com` Google accounts
- No need to enumerate individual projects

**Cons:**
- Grants access to **human users**, not project service accounts — the typical consumer of a VM image is a CI pipeline or infrastructure-as-code tool using a service account, not a human
- Does not require org membership — any `@redhat.com` account could use the image
- Security posture is unclear: makes the image visible to all Red Hat employees with GCP access

**Why not chosen:** The wrong principal type for automated VM creation workflows.

---

### Summary

| Model | Identifier | Requires GCP Org | Granularity | Chosen |
|---|---|---|---|---|
| **Project IDs** | `my-project-123` | No | Per-project | **Yes ✓** |
| Organization ID | `123456789012` | Yes | All projects in org | No |
| Folder ID | `987654321` | Yes | All projects in folder | No |
| Domain | `redhat.com` | No | All domain users | No |

**Current implementation uses project IDs.** If `aipcc-cicd` is ever moved under a Red Hat GCP organization, organization-level sharing can be added as a future enhancement — the `--share-orgs-ids` flag name already implies org semantics and could be extended to accept org IDs alongside project IDs.

---

### Note for Red Hat GCP Administrators

If you are evaluating whether to use project-level or org-level sharing, the key questions are:

1. Is `aipcc-cicd` (or the project hosting the images) part of a GCP organization? Check at `https://console.cloud.google.com/cloud-resource-manager`.
2. Do the consuming teams use consistent service accounts or Compute Engine default service agents?
3. Is there a standard GCP folder structure grouping related Red Hat projects?

Answers to these questions may inform a future revision of the sharing model.
