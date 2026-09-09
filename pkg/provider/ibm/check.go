package ibm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func (p *ibmProvider) ImageExists(imageName string) (bool, string, error) {
	apiKey := os.Getenv("IBMCLOUD_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("IC_API_KEY")
	}
	if apiKey == "" {
		return false, "", fmt.Errorf("IBMCLOUD_API_KEY is not set")
	}
	region, err := sourceRegion()
	if err != nil {
		return false, "", err
	}
	token, err := iamToken(apiKey)
	if err != nil {
		return false, "", fmt.Errorf("failed to get IAM token: %w", err)
	}
	return vpcImageByName(context.Background(), region, token, sanitizeImageName(imageName))
}

func iamToken(apiKey string) (string, error) {
	body := strings.NewReader(url.Values{
		"grant_type": {"urn:ibm:params:oauth:grant-type:apikey"},
		"apikey":     {apiKey},
	}.Encode())
	req, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "https://iam.cloud.ibm.com/identity/token", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IAM token request returned %d: %s", resp.StatusCode, raw)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return result.AccessToken, nil
}

func vpcImageByName(ctx context.Context, region, token, name string) (bool, string, error) {
	apiURL := fmt.Sprintf(
		"https://%s.iaas.cloud.ibm.com/v1/images?name=%s&visibility=private&version=2024-09-15",
		region, url.QueryEscape(name))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("VPC images API returned %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Images []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"images"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, "", err
	}
	if len(result.Images) > 0 {
		return true, result.Images[0].ID, nil
	}
	return false, "", nil
}
