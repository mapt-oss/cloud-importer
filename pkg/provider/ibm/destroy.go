package ibm

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mapt-oss/cloud-importer/pkg/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	catalogMgmtBeta     = "https://cm.globalcatalog.cloud.ibm.com/api/v1-beta"
	rcBaseURL           = "https://resource-controller.cloud.ibm.com"
	managedPollInterval = 10 * time.Second
	managedPollTimeout  = 5 * time.Minute
	// IBM needs a moment to register the reclamation in Resource Controller after a catalog delete.
	reclamationRegisterWait = 8 * time.Second
)

// PreDestroyCleanup must run before Pulumi destroys an IBM SNC/RHELAI register stack.
//
// IBM's catalog deletion goes into a "pending reclamation" state — it is not immediately
// hard-deleted. While reclamation is pending, the VPC IsImage still has
// catalog_offering.managed=true and IBM rejects any attempt to delete it with a 409.
//
// Correct order:
//  1. Delete the catalog via the Catalog Management API (cascades to offerings/versions).
//  2. Force-reclaim via the IBM Resource Controller API so reclamation completes immediately
//     instead of waiting for the default reclamation period (up to several hours).
//  3. Poll GET /v1/images/{id} until catalog_offering.managed becomes false.
//  4. Remove CmCatalog from Pulumi state so Pulumi skips its (already-done) deletion.
//
// Pulumi then destroys IsImage (now unmanaged), IamAuthorizationPolicy, and ResourceGroup.
func PreDestroyCleanup(projectName, stackName, backedURL string) error {
	apiKey := os.Getenv("IBMCLOUD_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("IBMCLOUD_API_KEY not set")
	}
	tok, err := iamAccessToken(apiKey)
	if err != nil {
		return fmt.Errorf("IAM token: %w", err)
	}

	ctx := gocontext.Background()
	s, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, nil, ibmStackOpts(projectName, backedURL)...)
	if err != nil {
		return fmt.Errorf("open stack: %w", err)
	}
	state, err := s.Export(ctx)
	if err != nil {
		return fmt.Errorf("export state: %w", err)
	}
	if len(state.Deployment) == 0 {
		return nil
	}
	var deployment map[string]interface{}
	if err := json.Unmarshal(state.Deployment, &deployment); err != nil {
		return fmt.Errorf("parse state: %w", err)
	}
	resources, _ := deployment["resources"].([]interface{})
	catalogID, imageID := ibmCatalogAndImageIDs(resources)

	if catalogID != "" {
		// Step 1: delete all offerings within the catalog before deleting the catalog itself.
		// Offering deletion is immediate (not soft-deleted) and removes catalog_offering.managed
		// from the VPC image without requiring a Resource Controller reclamation cycle.
		if err := deleteOfferingsHTTP(catalogID, tok); err != nil {
			logging.Warnf("pre-destroy: delete offerings in catalog %s: %v (continuing)", catalogID, err)
		}

		// Step 2: delete the catalog.
		if err := deleteCatalogHTTP(catalogID, tok); err != nil {
			logging.Warnf("pre-destroy: delete catalog %s: %v (may already be gone, continuing)", catalogID, err)
		} else {
			logging.Infof("pre-destroy: catalog %s deleted; waiting %s for reclamation to register",
				catalogID, reclamationRegisterWait)
			time.Sleep(reclamationRegisterWait)
		}

		// Step 3: force-reclaim so any RC soft-delete completes immediately.
		if err := forceReclaimResource(catalogID, tok); err != nil {
			logging.Warnf("pre-destroy: force-reclaim catalog %s: %v (image poll may timeout)", catalogID, err)
		}
	}

	// Step 3: poll the VPC image until catalog_offering.managed becomes false.
	if imageID != "" {
		region, err := sourceRegion()
		if err != nil {
			logging.Warnf("pre-destroy: cannot determine region for image poll: %v", err)
		} else if err := waitForImageUnmanaged(region, imageID, tok); err != nil {
			logging.Warnf("pre-destroy: %v (IsImage delete may still fail)", err)
		}
	}

	// Step 4: remove CmCatalog from Pulumi state — it is already deleted above.
	filtered := make([]interface{}, 0, len(resources))
	pruned := 0
	for _, r := range resources {
		res, ok := r.(map[string]interface{})
		if ok && res["type"] == cmCatalogType {
			pruned++
			continue
		}
		filtered = append(filtered, r)
	}
	if pruned == 0 {
		return nil
	}
	deployment["resources"] = filtered
	modified, err := json.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	state.Deployment = json.RawMessage(modified)
	if err := s.Import(ctx, state); err != nil {
		return fmt.Errorf("import state: %w", err)
	}
	logging.Infof("pre-destroy: removed CmCatalog from Pulumi state")
	return nil
}

const (
	cmCatalogType = "ibmcloud:index/cmCatalog:CmCatalog"
	isImageType   = "ibmcloud:index/isImage:IsImage"
)

type vpcImageCatalogOffering struct {
	Managed bool `json:"managed"`
}

type vpcImagePollResponse struct {
	CatalogOffering *vpcImageCatalogOffering `json:"catalog_offering"`
}

func ibmCatalogAndImageIDs(resources []interface{}) (catalogID, imageID string) {
	for _, r := range resources {
		res, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		switch res["type"] {
		case cmCatalogType:
			catalogID, _ = res["id"].(string)
		case isImageType:
			imageID, _ = res["id"].(string)
		}
	}
	return
}

// deleteOfferingsHTTP lists all offerings in a private catalog and deletes their
// versions before deleting the offerings themselves.
//
// IBM's documentation states: "To delete a custom image that is catalog managed,
// you must first delete the catalog offering version that is managing it."
// Deleting the catalog alone (which triggers a soft-delete/reclamation) is not
// sufficient — the version must be explicitly deleted to clear catalog_offering.managed
// on the VPC image.
func deleteOfferingsHTTP(catalogID, tok string) error {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/catalogs/%s/offerings", catalogMgmtBeta, catalogID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("list offerings HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Resources []struct {
			ID   string `json:"id"`
			Kinds []struct {
				Versions []struct {
					ID string `json:"id"`
				} `json:"versions"`
			} `json:"kinds"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse offerings response: %w", err)
	}

	for _, offering := range result.Resources {
		// Delete versions first — this is what actually clears catalog_offering.managed on the VPC image.
		for _, kind := range offering.Kinds {
			for _, ver := range kind.Versions {
				delReq, err := http.NewRequest(http.MethodDelete,
					fmt.Sprintf("%s/catalogs/%s/offerings/%s/versions/%s", catalogMgmtBeta, catalogID, offering.ID, ver.ID), nil)
				if err != nil {
					logging.Warnf("pre-destroy: build delete request for version %s: %v", ver.ID, err)
					continue
				}
				delReq.Header.Set("Authorization", "Bearer "+tok)
				delResp, err := http.DefaultClient.Do(delReq)
				if err != nil {
					logging.Warnf("pre-destroy: delete version %s: %v", ver.ID, err)
					continue
				}
				delResp.Body.Close()
				if delResp.StatusCode == http.StatusOK || delResp.StatusCode == http.StatusNoContent || delResp.StatusCode == http.StatusNotFound {
					logging.Infof("pre-destroy: offering version %s deleted", ver.ID)
				} else {
					logging.Warnf("pre-destroy: delete version %s: HTTP %d", ver.ID, delResp.StatusCode)
				}
			}
		}

		// Delete the offering itself.
		delReq, err := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("%s/catalogs/%s/offerings/%s", catalogMgmtBeta, catalogID, offering.ID), nil)
		if err != nil {
			logging.Warnf("pre-destroy: build delete request for offering %s: %v", offering.ID, err)
			continue
		}
		delReq.Header.Set("Authorization", "Bearer "+tok)
		delResp, err := http.DefaultClient.Do(delReq)
		if err != nil {
			logging.Warnf("pre-destroy: delete offering %s: %v", offering.ID, err)
			continue
		}
		delResp.Body.Close()
		if delResp.StatusCode == http.StatusOK || delResp.StatusCode == http.StatusNoContent || delResp.StatusCode == http.StatusNotFound {
			logging.Infof("pre-destroy: offering %s deleted", offering.ID)
		} else {
			logging.Warnf("pre-destroy: delete offering %s: HTTP %d", offering.ID, delResp.StatusCode)
		}
	}
	return nil
}

// deleteCatalogHTTP deletes a private catalog via the Catalog Management API.
// 404 is treated as success (already gone).
func deleteCatalogHTTP(catalogID, tok string) error {
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/catalogs/%s", catalogMgmtBeta, catalogID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}

// forceReclaimResource lists pending IBM Resource Controller reclamations for the
// given resource ID and immediately triggers each one so the soft-delete completes
// without waiting for the default reclamation window (which can be hours).
func forceReclaimResource(resourceID, tok string) error {
	req, err := http.NewRequest(http.MethodGet, rcBaseURL+"/v2/resource_instances/reclamations", nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	q.Set("resource_instance_id", resourceID)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Resources []struct {
			ID string `json:"id"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse reclamations response: %w", err)
	}
	if len(result.Resources) == 0 {
		logging.Infof("pre-destroy: no pending reclamations found for %s (may already be hard-deleted)", resourceID)
		return nil
	}
	for _, rec := range result.Resources {
		if err := runReclamationAction(rec.ID, "reclaim", tok); err != nil {
			logging.Warnf("pre-destroy: reclamation action on %s: %v", rec.ID, err)
		} else {
			logging.Infof("pre-destroy: forced immediate reclamation %s", rec.ID)
		}
	}
	return nil
}

func runReclamationAction(reclamationID, action, tok string) error {
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v2/reclamations/%s/actions/%s", rcBaseURL, reclamationID, action), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}

// waitForImageUnmanaged polls GET /v1/images/{imageID} until catalog_offering.managed
// is false (or the image is gone). Returns an error only if the timeout expires.
func waitForImageUnmanaged(region, imageID, tok string) error {
	getURL := fmt.Sprintf("https://%s.iaas.cloud.ibm.com/v1/images/%s?version=2024-01-01&generation=2",
		region, imageID)
	deadline := time.Now().Add(managedPollTimeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, getURL, nil)
		if err != nil {
			return fmt.Errorf("build poll request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("poll VPC image: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return nil // image already gone
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("poll VPC image: HTTP %d", resp.StatusCode)
		}
		var img vpcImagePollResponse
		if err := json.Unmarshal(body, &img); err != nil {
			return fmt.Errorf("parse VPC image response: %w", err)
		}
		if img.CatalogOffering == nil || !img.CatalogOffering.Managed {
			logging.Infof("pre-destroy: image %s is no longer catalog-managed", imageID)
			return nil
		}
		logging.Infof("pre-destroy: image %s still catalog-managed, retrying in %s...", imageID, managedPollInterval)
		time.Sleep(managedPollInterval)
	}
	return fmt.Errorf("timeout after %s: image %s is still catalog-managed", managedPollTimeout, imageID)
}

func ibmStackOpts(projectName, backedURL string) []auto.LocalWorkspaceOption {
	return []auto.LocalWorkspaceOption{
		auto.Project(workspace.Project{
			Name:    tokens.PackageName(projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Backend: &workspace.ProjectBackend{URL: backedURL},
		}),
		auto.WorkDir(filepath.Join(".")),
	}
}
