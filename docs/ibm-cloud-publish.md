# IBM Cloud — Catalog Publishing Workflow

IBM Cloud image import involves an extra step compared to other providers: after the VPC custom image is created, it must be linked to a **private catalog offering** and pre-released before it can be shared with external accounts. Part of this linking step must be done through the IBM Cloud web console (no API equivalent exists).

This document covers the full lifecycle for both SNC and RHEL AI images on IBM Cloud.

---

## Overview

The workflow has four phases:

```
1. cloud-importer snc ibm   →  VPC image + private catalog created
2. IBM Cloud Console (UI)   →  Share image to catalog offering (manual)
3. cloud-importer ibm publish  →  Pre-release version + share with accounts
4. cloud-importer destroy   →  Cleanup (catalog reclamation handled automatically)
```

Phases 1, 3, and 4 are fully automated. Phase 2 requires one manual step in the IBM Cloud web console.

---

## Prerequisites

Set the following environment variables before running any command:

| Variable | Description |
|---|---|
| `IBMCLOUD_API_KEY` | IBM Cloud API key |
| `IBMCLOUD_REGION` | IBM Cloud region (e.g. `us-south`). Legacy alias `IC_REGION` is also accepted. |
| `IBMCLOUD_COS_ACCESS_KEY` | HMAC access key for COS (used for upload and Pulumi state backend) |
| `IBMCLOUD_COS_SECRET_KEY` | HMAC secret key for COS |

---

## Phase 1 — Create the VPC image and private catalog

Run `cloud-importer snc ibm` (or `rhelai ibm`). This uploads the image to COS and creates two IBM Cloud resources via Pulumi:

- A **VPC custom image** (`IsImage`)
- A **private catalog** (`CmCatalog`) named after `--project-name`

```bash
podman run --rm --name import-snc-ibm -d \
    -e IBMCLOUD_API_KEY=${IBMCLOUD_API_KEY} \
    -e IBMCLOUD_REGION=${IBMCLOUD_REGION} \
    -e IBMCLOUD_COS_ACCESS_KEY=${IBMCLOUD_COS_ACCESS_KEY} \
    -e IBMCLOUD_COS_SECRET_KEY=${IBMCLOUD_COS_SECRET_KEY} \
    quay.io/aipcc-cicd/cloud-importer:latest snc ibm \
        --project-name "snc-4.20.0" \
        --backed-url cos://aipcc-productization/cloud-importer \
        --bundle-uri ${BUNDLE_URL} \
        --shasum-uri ${SHASUM_URL} \
        --arch ${ARCH} \
        --debug \
        --debug-level 9

podman logs -f import-snc-ibm
```

At the end of a successful run, four stack outputs are printed:

```
catalogID:    <catalog-id>
catalogName:  snc-4.20.0
vpcImageID:   <vpc-image-id>
vpcImageName: snc-4.20.0
```

Save these values — you will need `catalogName` and `vpcImageName` for the UI step.

---

## Phase 2 — Link the image to the catalog (IBM Cloud Console)

This step creates the catalog **offering** and **version** and establishes the bidirectional link between the VPC custom image and the catalog. There is no public API for this action; it must be done through the IBM Cloud web console.

1. Log in to [cloud.ibm.com](https://cloud.ibm.com)
2. Navigate to **VPC Infrastructure → Compute → Images → Custom images**
3. Find the image named `<vpcImageName>` (e.g. `snc-4.20.0`)
4. Click **Actions → Share to catalog**
5. Select the catalog named `<catalogName>` (e.g. `snc-4.20.0`)
6. Enter a version string (e.g. `4.20.0`)
7. Click **Create catalog offering**
8. Click **Validate image** and wait for validation to complete

After validation, navigate to **Catalog → Private catalogs → `<catalogName>` → Offerings** and open the offering. Copy the **version locator** — it looks like:

```
<catalogID>.<versionID>
```

You will use this in Phase 3.

---

## Phase 3 — Pre-release and share with accounts

Run `cloud-importer ibm publish` with the version locator from Phase 2 and the list of IBM account IDs that should have access:

```bash
cloud-importer ibm publish \
    --version-locator <catalogID>.<versionID> \
    --share-orgs-ids account-id-1,account-id-2,account-id-3
```

This command:

1. **Pre-releases** the version (`PrereleaseVersion`) — makes it deployable within the owning account
2. **Sets the offering to `allow_request` mode** — allows external accounts to be granted access
3. **Reconciles the share approval list** — adds accounts not yet on the list, removes accounts no longer in the list

The command is **idempotent**: run it again at any time to add or remove accounts. The `--share-orgs-ids` flag is the full desired state, not a delta.

### Flags

| Flag | Required | Description |
|---|---|---|
| `--version-locator` | Yes | Version locator string from the IBM Cloud catalog UI (`catalogID.versionID`) |
| `--share-orgs-ids` | No | Comma-separated list of IBM account IDs to share with. Omit or leave empty to clear all accounts. |

### Example — updating the account list

```bash
# Initial publish to two accounts
cloud-importer ibm publish \
    --version-locator abc123.def456 \
    --share-orgs-ids account-a,account-b

# Later: add a third account (previous two remain, third is added)
cloud-importer ibm publish \
    --version-locator abc123.def456 \
    --share-orgs-ids account-a,account-b,account-c

# Remove account-b (account-a and account-c remain)
cloud-importer ibm publish \
    --version-locator abc123.def456 \
    --share-orgs-ids account-a,account-c
```

---

## Phase 4 — Destroy

When you no longer need the image, destroy all resources with the same `--project-name` and `--backed-url` used during import.

```bash
podman run --rm \
    -e IBMCLOUD_API_KEY=${IBMCLOUD_API_KEY} \
    -e IBMCLOUD_REGION=${IBMCLOUD_REGION} \
    -e IBMCLOUD_COS_ACCESS_KEY=${IBMCLOUD_COS_ACCESS_KEY} \
    -e IBMCLOUD_COS_SECRET_KEY=${IBMCLOUD_COS_SECRET_KEY} \
    quay.io/aipcc-cicd/cloud-importer:latest destroy \
        --type snc \
        --project-name "snc-4.20.0" \
        --backed-url cos://aipcc-productization/cloud-importer
```

### Why destroy is different on IBM Cloud

IBM Cloud does not allow deleting a VPC custom image that is linked to a catalog offering (`catalog_offering.managed = true`). Catalog deletion is also asynchronous — IBM soft-deletes catalogs into a pending-reclamation state before actually removing them, and the VPC image remains catalog-managed during that window.

`cloud-importer` handles this automatically before invoking Pulumi destroy:

1. **Delete the catalog** via the Catalog Management API
2. **Wait** for the reclamation record to register (~8 seconds)
3. **Force-reclaim** the catalog via the Resource Controller API (bypasses the 7-day soft-delete window)
4. **Poll the VPC image** until `catalog_offering.managed` becomes `false` (up to 5 minutes)
5. **Prune the catalog resource** from Pulumi state so Pulumi does not attempt to delete it again
6. **Run Pulumi destroy** — the VPC image is now unmanaged and can be deleted normally

If destroy fails partway through, re-run with `--force-destroy` to clear any Pulumi lock before retrying.

---

## Troubleshooting

**`cloud-importer ibm publish` returns "version not found"**

The version locator must come from the IBM Cloud catalog UI after the UI sharing step in Phase 2. Check that you have completed Phase 2 and are copying the locator from **Catalog → Private catalogs → `<catalogName>` → Offerings → `<offeringName>` → version row**.

**Destroy fails with "catalog-managed" error on the VPC image**

The pre-destroy catalog reclamation step did not complete. Run destroy again — on retry the catalog may already be deleted and the image may already be unmanaged. If it still fails, check the IBM Cloud resource reclamation list:

```bash
ibmcloud resource reclamations
ibmcloud resource reclamation-delete <reclamation-id>
```

Then re-run destroy.

**ResourceGroup deletion takes a very long time**

This is normal IBM Cloud behavior. IBM Cloud verifies that all resources in the group are deleted before removing the group itself, and this check can take 10–30 minutes. Leave the process running.

**`ibm publish` — `SetOfferingPublish` returns a non-fatal error**

The `allow_request` mode may already be set from a previous run. The command logs a warning and continues. This does not affect the share approval list reconciliation.
