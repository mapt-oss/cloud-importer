package ibm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/catalogmanagementv1"
	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
)

const (
	catalogVersionPollInterval = 15 * time.Second
	catalogVersionPollTimeout  = 15 * time.Minute
	catalogValidationTimeout   = 30 * time.Minute // VSI validation spins up a test instance
)

// catalogValidationInvalidError is returned specifically when IBM's catalog validation
// completes with state "invalid" — distinct from infrastructure errors during setup.
type catalogValidationInvalidError struct{ msg string }

func (e *catalogValidationInvalidError) Error() string {
	return "catalog validation failed" + e.msg
}

// PublishVPCImage creates (or reuses) a catalog offering for the VPC image,
// drives the version through new → validated → prerelease → consumable, and
// reconciles the per-offering account allowlist.
//
// Idempotent: if the version already exists in consumable state, only the
// allowlist is updated.
func PublishVPCImage(catalogID, offeringName, vpcImageID, vpcImageName, version, region string, shareOrgIds []string) error {
	apiKey := os.Getenv("IBMCLOUD_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("IBMCLOUD_API_KEY not set")
	}
	client, err := catalogmanagementv1.NewCatalogManagementV1(&catalogmanagementv1.CatalogManagementV1Options{
		Authenticator: &core.IamAuthenticator{ApiKey: apiKey},
	})
	if err != nil {
		return fmt.Errorf("create catalog management client: %w", err)
	}

	// Fetch IAM token once — needed for VPC API calls during import and validation.
	token, err := iamToken(apiKey)
	if err != nil {
		return fmt.Errorf("get IAM token: %w", err)
	}

	offeringID, versionLocator, versionState, err := findOrCreateOffering(client, catalogID, offeringName, version)
	if err != nil {
		return err
	}

	// Import only if the version doesn't exist yet.
	if versionLocator == "" {
		versionLocator, err = importVersion(client, catalogID, offeringID, version, vpcImageID, vpcImageName, region, token)
		if err != nil {
			return fmt.Errorf("import offering version: %w", err)
		}
		versionState = "new"
		logging.Infof("publish: imported version %s (locator: %s)", version, versionLocator)
	} else {
		logging.Infof("publish: version %s already exists in state %q — resuming", version, versionState)
	}

	// Drive the version through the lifecycle, resuming from wherever it currently is.
	if versionState == "consumable" {
		logging.Infof("publish: version %s already consumable — updating allowlist only", version)
		return reconcileOfferingAccessList(client, catalogID, offeringID, shareOrgIds)
	}

	// A version already in prerelease: try consumable, handle the expected "not consumable"
	// (unvalidated) response gracefully — prerelease is sufficient for private catalog sharing.
	if versionState == "prerelease" {
		_, cerr := client.ConsumableVersion(client.NewConsumableVersionOptions(versionLocator))
		if cerr == nil || strings.Contains(cerr.Error(), "current state consumable") {
			goto waitConsumable
		}
		if strings.Contains(cerr.Error(), "not consumable") {
			logging.Infof("publish: version %s in prerelease — consumable requires validation, staying in prerelease for private catalog sharing", version)
			return reconcileOfferingAccessList(client, catalogID, offeringID, shareOrgIds)
		}
		return fmt.Errorf("make version consumable: %w", cerr)
	}

	// For private catalog offerings (not published to IBM Cloud's public catalog),
	// Schematics-based validation is optional. We attempt prerelease directly and fall
	// back to the full validation flow only if IBM rejects the transition.
	if versionState == "new" || versionState == "validated" {
		if _, err := client.PrereleaseVersion(client.NewPrereleaseVersionOptions(versionLocator)); err != nil {
			if strings.Contains(err.Error(), "current state prerelease") {
				// Already there — continue.
			} else if versionState == "new" && !strings.Contains(err.Error(), "current state prerelease") {
				// IBM requires validation before prerelease for this offering type.
				// Fall back to the full Schematics validation flow.
				logging.Infof("publish: direct prerelease rejected (%v) — running Schematics validation", err)
				if verr := triggerAndWaitVSIValidation(apiKey, client, versionLocator); verr != nil {
					var valErr *catalogValidationInvalidError
					if errors.As(verr, &valErr) {
						if _, derr := client.DeleteVersion(client.NewDeleteVersionOptions(versionLocator)); derr != nil {
							logging.Warnf("publish: could not delete failed version %s: %v", versionLocator, derr)
						} else {
							logging.Infof("publish: deleted failed version %s — re-run to reimport and retry", versionLocator)
						}
					}
					return fmt.Errorf("vsi validation: %w", verr)
				}
				if _, err2 := client.PrereleaseVersion(client.NewPrereleaseVersionOptions(versionLocator)); err2 != nil {
					if !strings.Contains(err2.Error(), "current state prerelease") {
						return fmt.Errorf("prerelease version after validation: %w", err2)
					}
				}
			} else {
				return fmt.Errorf("prerelease version: %w", err)
			}
		}
		logging.Infof("publish: version %s set to prerelease", versionLocator)
		if _, err := client.ConsumableVersion(client.NewConsumableVersionOptions(versionLocator)); err != nil {
			switch {
			case strings.Contains(err.Error(), "current state consumable"):
				// already consumable — nothing to do
			case strings.Contains(err.Error(), "not consumable"):
				// IBM requires Schematics validation before consumable; prerelease is sufficient
				// for private catalog offerings shared via account access list.
				logging.Infof("publish: version %s stays in prerelease (consumable requires validation — not needed for private catalog sharing)", versionLocator)
				return reconcileOfferingAccessList(client, catalogID, offeringID, shareOrgIds)
			default:
				return fmt.Errorf("make version consumable: %w", err)
			}
		}
		logging.Infof("publish: version %s set to consumable", versionLocator)
	}

waitConsumable:
	if err := waitForVersionState(client, versionLocator, "consumable"); err != nil {
		return fmt.Errorf("wait for consumable: %w", err)
	}
	logging.Infof("publish: version %s is consumable", versionLocator)
	return reconcileOfferingAccessList(client, catalogID, offeringID, shareOrgIds)
}

// findOrCreateOffering returns (offeringID, versionLocator, versionState, error).
// versionLocator is non-empty whenever the version already exists (any state).
// versionState is the current state string ("new", "validated", "prerelease", "consumable", …) or "" if not found.
func findOrCreateOffering(client *catalogmanagementv1.CatalogManagementV1, catalogID, name, version string) (string, string, string, error) {
	result, _, err := client.ListOfferings(
		client.NewListOfferingsOptions(catalogID).SetName(name),
	)
	if err != nil {
		return "", "", "", fmt.Errorf("list offerings in catalog %s: %w", catalogID, err)
	}
	for _, o := range result.Resources {
		if ptrStr(o.Name) != name {
			continue
		}
		offeringID := ptrStr(o.ID)
		for _, kind := range o.Kinds {
			for _, ver := range kind.Versions {
				if ptrStr(ver.Version) == version {
					state := ""
					if ver.State != nil {
						state = ptrStr(ver.State.Current)
					}
					return offeringID, ptrStr(ver.VersionLocator), state, nil
				}
			}
		}
		return offeringID, "", "", nil
	}

	offering, _, err := client.CreateOffering(
		client.NewCreateOfferingOptions(catalogID).SetName(name).SetLabel(name),
	)
	if err != nil {
		return "", "", "", fmt.Errorf("create offering %q in catalog %s: %w", name, catalogID, err)
	}
	logging.Infof("publish: created offering %q (%s)", name, ptrStr(offering.ID))
	return ptrStr(offering.ID), "", "", nil
}

// importVersion imports a VPC image as a new catalog offering version.
// Image metadata is sent directly in the request body — no COS upload required.
func importVersion(client *catalogmanagementv1.CatalogManagementV1, catalogID, offeringID, version, vpcImageID, vpcImageName, region, token string) (string, error) {
	// Fetch VPC image details — IBM requires operating_system, file.size, and
	// minimum_provisioned_size to generate the Schematics installer template during validation.
	imgMeta, err := fetchVPCImageMeta(context.Background(), region, token, vpcImageID)
	if err != nil {
		return "", fmt.Errorf("fetch VPC image metadata: %w", err)
	}

	imgOS := imgMeta.OperatingSystem
	meta := &catalogmanagementv1.ImportOfferingBodyMetadata{
		OperatingSystem: &catalogmanagementv1.ImportOfferingBodyMetadataOperatingSystem{
			DedicatedHostOnly: core.BoolPtr(imgOS.DedicatedHostOnly),
			Vendor:            core.StringPtr(imgOS.Vendor),
			Name:              core.StringPtr(imgOS.Name),
			Href:              core.StringPtr(imgOS.Href),
			DisplayName:       core.StringPtr(imgOS.DisplayName),
			Family:            core.StringPtr(imgOS.Family),
			Version:           core.StringPtr(imgOS.Version),
			Architecture:      core.StringPtr(imgOS.Architecture),
		},
		File:                   &catalogmanagementv1.ImportOfferingBodyMetadataFile{Size: core.Int64Ptr(imgMeta.File.Size)},
		MinimumProvisionedSize: core.Int64Ptr(imgMeta.MinimumProvisionedSize),
		Images: []catalogmanagementv1.ImportOfferingBodyMetadataImagesItem{
			{
				ID:     core.StringPtr(vpcImageID),
				Name:   core.StringPtr(vpcImageName),
				Region: core.StringPtr(region),
			},
		},
	}
	// IBM's API requires a sha field even for inline VPC image imports.
	// We compute it as the SHA256 of the metadata JSON, which is deterministic
	// and unique per image.
	metaJSON, _ := json.Marshal(meta)
	sha := fmt.Sprintf("%x", sha256.Sum256(metaJSON))

	opts := client.NewImportOfferingVersionOptions(catalogID, offeringID).
		SetName(version).
		SetLabel(version).
		SetVersion(version).
		SetInstallKind("instance").
		SetTargetKinds([]string{"vpc-x86"}).
		SetFormatKind("vsi-image").
		SetSha(sha).
		SetMetadata(meta).
		SetIsVsi(true).
		SetIncludeConfig(true)
	offering, _, err := client.ImportOfferingVersion(opts)
	if err != nil {
		return "", err
	}
	for _, kind := range offering.Kinds {
		for _, ver := range kind.Versions {
			if ptrStr(ver.Version) == version {
				return ptrStr(ver.VersionLocator), nil
			}
		}
	}
	return "", fmt.Errorf("version %s not found in ImportOfferingVersion response", version)
}

// iamRefreshToken exchanges the API key for a token pair and returns the refresh token.
// ValidateInstall requires an X-Auth-Refresh-Token header (not a bearer token).
func iamRefreshToken(apiKey string) (string, error) {
	auth := &core.IamAuthenticator{ApiKey: apiKey}
	if _, err := auth.GetToken(); err != nil {
		return "", fmt.Errorf("get IAM token: %w", err)
	}
	if auth.RefreshToken == "" {
		return "", fmt.Errorf("IAM token exchange did not return a refresh token")
	}
	return auth.RefreshToken, nil
}

const defaultValidationProfile = "bx2-8x32"

// vsiValidationOverrides builds the DeployRequestBodyOverrideValues IBM needs to spin up
// a test VSI instance during catalog validation using the provided ephemeral IDs.
func vsiValidationOverrides(instanceName, vpcID, subnetID, zone, region string) *catalogmanagementv1.DeployRequestBodyOverrideValues {
	return &catalogmanagementv1.DeployRequestBodyOverrideValues{
		VsiInstanceName: core.StringPtr(instanceName),
		VPCID:           core.StringPtr(vpcID),
		SubnetID:        core.StringPtr(subnetID),
		SubnetZone:      core.StringPtr(zone),
		VPCProfile:      core.StringPtr(defaultValidationProfile),
		VPCRegion:       core.StringPtr(region),
	}
}

// triggerAndWaitVSIValidation submits a catalog validation job for a vsi-image version
// (equivalent to the UI "Validate" button) and polls until the job reaches "valid" or "invalid".
// It creates an ephemeral VPC + subnet for the test VSI and tears them down afterwards.
func triggerAndWaitVSIValidation(apiKey string, client *catalogmanagementv1.CatalogManagementV1, versionLocator string) error {
	region, err := sourceRegion()
	if err != nil {
		return err
	}

	token, err := iamToken(apiKey)
	if err != nil {
		return fmt.Errorf("get IAM token for ephemeral VPC: %w", err)
	}
	refreshToken, err := iamRefreshToken(apiKey)
	if err != nil {
		return err
	}

	// Derive a unique name from the version-specific UUID (the part after the first ".").
	// The catalog-ID prefix is constant across reimports and would produce the same VPC name,
	// causing 409 conflicts if a previous cleanup left the VPC behind.
	parts := strings.SplitN(versionLocator, ".", 2)
	versionPart := parts[len(parts)-1]
	rawName := strings.ToLower(versionPart)
	if len(rawName) > 32 {
		rawName = rawName[:32]
	}
	rawName = strings.TrimRight(rawName, "-")
	resourceName := "ci-val-" + rawName
	instanceName := resourceName

	ctx := context.Background()

	logging.Infof("publish: creating ephemeral VPC %q for catalog validation", resourceName)
	vpcID, err := createEphemeralVPC(ctx, region, token, resourceName)
	if err != nil {
		return fmt.Errorf("create ephemeral VPC: %w", err)
	}

	zone := region + "-1"
	logging.Infof("publish: creating ephemeral subnet in zone %s", zone)
	subnetID, err := createEphemeralSubnet(ctx, region, token, vpcID, zone, resourceName)
	if err != nil {
		// VPC was created — clean it up before returning.
		if delErr := deleteEphemeralVPC(ctx, region, token, vpcID); delErr != nil {
			logging.Warnf("publish: cleanup: delete ephemeral VPC %s: %v", vpcID, delErr)
		}
		return fmt.Errorf("create ephemeral subnet: %w", err)
	}

	// Always clean up the ephemeral resources, whether validation succeeds or not.
	defer func() {
		logging.Infof("publish: cleaning up ephemeral subnet %s", subnetID)
		if err := deleteEphemeralSubnet(ctx, region, token, subnetID); err != nil {
			logging.Warnf("publish: cleanup: delete ephemeral subnet %s: %v", subnetID, err)
		}
		logging.Infof("publish: cleaning up ephemeral VPC %s", vpcID)
		if err := deleteEphemeralVPC(ctx, region, token, vpcID); err != nil {
			logging.Warnf("publish: cleanup: delete ephemeral VPC %s: %v", vpcID, err)
		}
	}()

	overrides := vsiValidationOverrides(instanceName, vpcID, subnetID, zone, region)
	opts := client.NewValidateInstallOptions(versionLocator, refreshToken).
		SetOverrideValues(overrides)
	if _, err := client.ValidateInstall(opts); err != nil {
		return fmt.Errorf("ValidateInstall: %w", err)
	}
	logging.Infof("publish: validation job submitted for %s (test instance: %s)", versionLocator, instanceName)

	deadline := time.Now().Add(catalogValidationTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(catalogVersionPollInterval)
		status, _, err := client.GetValidationStatus(client.NewGetValidationStatusOptions(versionLocator, refreshToken))
		if err != nil {
			return fmt.Errorf("GetValidationStatus: %w", err)
		}
		state := ""
		if status != nil && status.State != nil {
			state = *status.State
		}
		msg := ""
		if status != nil && status.Message != nil && *status.Message != "" {
			msg = " — " + *status.Message
		}
		logging.Infof("publish: validation state %q%s", state, msg)
		switch state {
		case "valid":
			return nil
		case "invalid":
			// Log raw status JSON for debugging when IBM doesn't populate Message.
			if raw, jerr := json.Marshal(status); jerr == nil {
				logging.Infof("publish: validation status detail: %s", raw)
			}
			return &catalogValidationInvalidError{msg: msg}
		}
	}
	return fmt.Errorf("timeout after %s: validation did not complete for %s", catalogValidationTimeout, versionLocator)
}

// waitForVersionState polls GetVersion until the version's State.Current equals wantState.
func waitForVersionState(client *catalogmanagementv1.CatalogManagementV1, versionLocator, wantState string) error {
	deadline := time.Now().Add(catalogVersionPollTimeout)
	for time.Now().Before(deadline) {
		offering, _, err := client.GetVersion(client.NewGetVersionOptions(versionLocator))
		if err != nil {
			return fmt.Errorf("get version %s: %w", versionLocator, err)
		}
		if current, found := versionCurrentState(offering, versionLocator); found {
			if current == wantState {
				return nil
			}
			logging.Infof("publish: version state is %q, waiting for %q...", current, wantState)
		}
		time.Sleep(catalogVersionPollInterval)
	}
	return fmt.Errorf("timeout after %s: version %s did not reach state %q", catalogVersionPollTimeout, versionLocator, wantState)
}

func versionCurrentState(offering *catalogmanagementv1.Offering, versionLocator string) (string, bool) {
	for _, kind := range offering.Kinds {
		for _, ver := range kind.Versions {
			if ptrStr(ver.VersionLocator) != versionLocator {
				continue
			}
			if ver.State != nil {
				return ptrStr(ver.State.Current), true
			}
			return "", true
		}
	}
	return "", false
}

// reconcileOfferingAccessList brings the per-offering account access list in sync
// with shareOrgIds. Idempotent: existing accounts not in shareOrgIds are removed,
// missing ones are added.
func reconcileOfferingAccessList(client *catalogmanagementv1.CatalogManagementV1, catalogID, offeringID string, shareOrgIds []string) error {
	if len(shareOrgIds) == 0 {
		logging.Infof("publish: no share targets specified, skipping access list reconciliation")
		return nil
	}

	desired := make(map[string]bool, len(shareOrgIds))
	for _, id := range shareOrgIds {
		desired[id] = true
	}

	current := make(map[string]bool)
	listOpts := client.NewGetOfferingAccessListOptions(catalogID, offeringID)
	listOpts.SetLimit(100)
	for {
		result, _, err := client.GetOfferingAccessList(listOpts)
		if err != nil {
			return fmt.Errorf("get offering access list: %w", err)
		}
		for _, a := range result.Resources {
			if a.Account != nil && *a.Account != "" {
				current[*a.Account] = true
			}
		}
		if result.Next == nil || result.Next.Start == nil {
			break
		}
		listOpts.SetStart(*result.Next.Start)
	}

	var toAdd, toRemove []string
	for id := range desired {
		if !current[id] {
			toAdd = append(toAdd, id)
		}
	}
	for id := range current {
		if !desired[id] {
			toRemove = append(toRemove, id)
		}
	}

	// IBM catalog API silently drops accounts beyond a small batch size, so
	// add one account at a time to guarantee each is registered.
	for _, id := range toAdd {
		if _, _, err := client.AddOfferingAccessList(
			client.NewAddOfferingAccessListOptions(catalogID, offeringID, []string{id}),
		); err != nil {
			return fmt.Errorf("add account %s to offering access list: %w", id, err)
		}
		logging.Infof("publish: added account %s to offering access list", id)
	}
	if len(toRemove) > 0 {
		if _, _, err := client.DeleteOfferingAccessList(
			client.NewDeleteOfferingAccessListOptions(catalogID, offeringID, toRemove),
		); err != nil {
			return fmt.Errorf("remove accounts from offering access list: %w", err)
		}
		logging.Infof("publish: removed %d account(s) from offering access list", len(toRemove))
	}
	if len(toAdd) == 0 && len(toRemove) == 0 {
		logging.Infof("publish: offering access list already up to date (%d account(s))", len(current))
		return nil
	}
	// Enable access-list visibility so newly added accounts receive a share
	// request. Only called when the list changed — IBM blocks this call while
	// there are pending approvals, so we skip it on no-op runs.
	// Best-effort: log a warning if the account is not configured for the
	// approval flow rather than failing the whole publish step.
	if len(toAdd) > 0 {
		if _, _, err := client.ShareOffering(
			client.NewShareOfferingOptions(catalogID, offeringID).SetEnabled(true),
		); err != nil {
			logging.Warnf("publish: could not enable offering access-list sharing (account may not be configured for approval flow): %v", err)
		} else {
			logging.Infof("publish: offering sharing enabled for %d new account(s)", len(toAdd))
		}
	}
	return nil
}


func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
