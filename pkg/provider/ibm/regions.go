package ibm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// listVPCRegions returns all available IBM VPC region names.
// Any regional endpoint can list all regions; us-south is used as the bootstrap.
func listVPCRegions(ctx context.Context, token string) ([]string, error) {
	url := fmt.Sprintf("https://us-south.iaas.cloud.ibm.com/v1/regions?version=%s&generation=2", vpcAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list VPC regions: status %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		Regions []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"regions"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse VPC regions: %w", err)
	}
	names := make([]string, 0, len(result.Regions))
	for _, r := range result.Regions {
		if r.Status == "available" {
			names = append(names, r.Name)
		}
	}
	return names, nil
}

// otherVPCRegions returns all available IBM VPC regions except primaryRegion,
// limited to regions that have a corresponding IBM COS regional endpoint.
// VPC regions without COS support (e.g. eu-es, ca-mon) are excluded because
// replica buckets cannot be created there.
func otherVPCRegions(primaryRegion, token string) ([]string, error) {
	all, err := listVPCRegions(context.Background(), token)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(all))
	for _, r := range all {
		if r != primaryRegion && ibmCOSSupportedRegions[r] {
			out = append(out, r)
		}
	}
	return out, nil
}
