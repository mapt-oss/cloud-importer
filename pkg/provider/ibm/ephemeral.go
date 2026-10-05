package ibm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const vpcAPIVersion = "2024-09-15"

func vpcAPIBase(region string) string {
	return fmt.Sprintf("https://%s.iaas.cloud.ibm.com/v1", region)
}

// createEphemeralVPC creates a minimal VPC for catalog validation and returns its ID.
func createEphemeralVPC(ctx context.Context, region, token, name string) (string, error) {
	body, _ := json.Marshal(map[string]any{"name": name})
	url := fmt.Sprintf("%s/vpcs?version=%s&generation=2", vpcAPIBase(region), vpcAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create VPC: status %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse VPC response: %w", err)
	}
	return result.ID, nil
}

// createEphemeralSubnet creates a subnet inside vpcID and returns its ID.
func createEphemeralSubnet(ctx context.Context, region, token, vpcID, zone, name string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"name":                    name,
		"vpc":                     map[string]string{"id": vpcID},
		"zone":                    map[string]string{"name": zone},
		"total_ipv4_address_count": 256,
	})
	url := fmt.Sprintf("%s/subnets?version=%s&generation=2", vpcAPIBase(region), vpcAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create subnet: status %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse subnet response: %w", err)
	}
	return result.ID, nil
}

// deleteEphemeralSubnet deletes a subnet by ID. Best-effort: errors are logged, not fatal.
func deleteEphemeralSubnet(ctx context.Context, region, token, subnetID string) error {
	url := fmt.Sprintf("%s/subnets/%s?version=%s&generation=2", vpcAPIBase(region), subnetID, vpcAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete subnet %s: status %d: %s", subnetID, resp.StatusCode, raw)
	}
	return nil
}


// deleteEphemeralVPC deletes a VPC by ID. Best-effort: errors are logged, not fatal.
func deleteEphemeralVPC(ctx context.Context, region, token, vpcID string) error {
	url := fmt.Sprintf("%s/vpcs/%s?version=%s&generation=2", vpcAPIBase(region), vpcID, vpcAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete VPC %s: status %d: %s", vpcID, resp.StatusCode, raw)
	}
	return nil
}
