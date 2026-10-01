package ibm

import (
	"fmt"
	"os"
	"strings"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/catalogmanagementv1"
	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
)

// PublishCatalogVersion pre-releases a catalog version and reconciles the
// share approval list so exactly the given account IDs have access.
//
// It is idempotent: calling it again with the same or updated account list
// adds new entries and removes stale ones. The passed list is the full desired
// state, not a delta.
//
// Call this after the operator has completed the UI sharing step:
//
//	VPC → Compute → Images → <image> → Actions → Share to catalog → Validate image
func PublishCatalogVersion(versionLocator string, accountIDs []string) error {
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

	// Retrieve version metadata so we have catalogID + offeringID.
	ver, _, err := client.GetVersion(client.NewGetVersionOptions(versionLocator))
	if err != nil {
		return fmt.Errorf("get version %s: %w", versionLocator, err)
	}
	catalogID := ptrStr(ver.CatalogID)
	offeringID := ptrStr(ver.ID)
	if catalogID == "" || offeringID == "" {
		return fmt.Errorf("version %s: missing catalog_id or offering_id in response", versionLocator)
	}

	// Step 1: pre-release the version (makes it deployable within this account).
	// IBM returns an error if the version is already in prerelease state — treat that as a no-op.
	if _, err := client.PrereleaseVersion(
		client.NewPrereleaseVersionOptions(versionLocator),
	); err != nil {
		if !strings.Contains(err.Error(), "current state prerelease") {
			return fmt.Errorf("pre-release version %s: %w", versionLocator, err)
		}
		logging.Infof("publish: version %s already in prerelease state, continuing", versionLocator)
	} else {
		logging.Infof("publish: version %s pre-released", versionLocator)
	}

	// Step 2: set offering to allow-request mode so external accounts can be granted access.
	if _, _, err := client.SetOfferingPublish(
		client.NewSetOfferingPublishOptions(catalogID, offeringID, "publish_approved", "true"),
	); err != nil {
		// Non-fatal: may already be in allow_request mode.
		logging.Warnf("publish: set offering publish mode: %v (continuing)", err)
	} else {
		logging.Infof("publish: offering %s/%s set to allow_request mode", catalogID, offeringID)
	}

	// Step 3: reconcile the share approval list against the desired account IDs.
	if err := reconcileShareApprovalList(client, accountIDs); err != nil {
		return fmt.Errorf("reconcile share approval list: %w", err)
	}
	return nil
}

// reconcileShareApprovalList fetches the full current share approval list and
// brings it in sync with desiredIDs. Only entries we manage (raw account IDs
// from previous publish runs) are removed; IBM-internal entries are left intact.
func reconcileShareApprovalList(client *catalogmanagementv1.CatalogManagementV1, desiredIDs []string) error {
	desired := make(map[string]bool, len(desiredIDs))
	for _, id := range desiredIDs {
		desired[id] = true
	}

	// Fetch all pages of the current approval list.
	current, err := fetchAllApprovalAccounts(client)
	if err != nil {
		return fmt.Errorf("fetch current list: %w", err)
	}

	var toAdd, toRemove []string
	for id := range desired {
		if !current[id] {
			toAdd = append(toAdd, "-acct-"+id)
		}
	}
	for id := range current {
		if !desired[id] {
			toRemove = append(toRemove, "-acct-"+id)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		logging.Infof("publish: share approval list already up to date (%d account(s))", len(current))
		return nil
	}
	if len(toAdd) > 0 {
		if _, _, err := client.AddShareApprovalList(
			client.NewAddShareApprovalListOptions("offering", toAdd),
		); err != nil {
			return fmt.Errorf("add accounts: %w", err)
		}
		logging.Infof("publish: added %d account(s) to share approval list", len(toAdd))
	}
	if len(toRemove) > 0 {
		if _, _, err := client.DeleteShareApprovalList(
			client.NewDeleteShareApprovalListOptions("offering", toRemove),
		); err != nil {
			return fmt.Errorf("remove accounts: %w", err)
		}
		logging.Infof("publish: removed %d stale account(s) from share approval list", len(toRemove))
	}
	return nil
}

// fetchAllApprovalAccounts pages through GetShareApprovalList and returns a set
// of raw account IDs (without the "-acct-" prefix) currently on the list.
func fetchAllApprovalAccounts(client *catalogmanagementv1.CatalogManagementV1) (map[string]bool, error) {
	accounts := make(map[string]bool)
	opts := client.NewGetShareApprovalListOptions("offering")
	opts.SetLimit(100)
	for {
		result, _, err := client.GetShareApprovalList(opts)
		if err != nil {
			return nil, err
		}
		for _, entry := range result.Resources {
			if entry.Account != nil && *entry.Account != "" {
				accounts[*entry.Account] = true
			}
		}
		if result.Next == nil || result.Next.Start == nil {
			break
		}
		opts.SetStart(*result.Next.Start)
	}
	return accounts, nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
