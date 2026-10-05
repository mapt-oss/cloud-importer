package ibm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/iampolicymanagementv1"
)

const (
	iamTokenURL = "https://iam.cloud.ibm.com/identity/token"
)

func iamAccessToken(apiKey string) (string, error) {
	resp, err := http.PostForm(iamTokenURL, url.Values{
		"grant_type": {"urn:ibm:params:oauth:grant-type:apikey"},
		"apikey":     {apiKey},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access_token from IAM")
	}
	return result.AccessToken, nil
}

// accountIDFromJWT extracts the account ID from the `account.bss` claim of an IBM IAM JWT.
func accountIDFromJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("unexpected JWT format")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims struct {
		Account struct {
			BSS string `json:"bss"`
		} `json:"account"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", fmt.Errorf("parse JWT claims: %w", err)
	}
	if claims.Account.BSS == "" {
		return "", fmt.Errorf("account ID (account.bss) not found in JWT")
	}
	return claims.Account.BSS, nil
}

// findVPCCOSAuthPolicy returns the ID of an existing VPC→COS Reader authorization
// policy in the account, or "" if none is found.
func findVPCCOSAuthPolicy() (string, error) {
	apiKey := os.Getenv("IBMCLOUD_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("IBMCLOUD_API_KEY not set")
	}

	tok, err := iamAccessToken(apiKey)
	if err != nil {
		return "", fmt.Errorf("IAM token: %w", err)
	}
	accountID, err := accountIDFromJWT(tok)
	if err != nil {
		return "", fmt.Errorf("account ID from token: %w", err)
	}

	client, err := iampolicymanagementv1.NewIamPolicyManagementV1(&iampolicymanagementv1.IamPolicyManagementV1Options{
		Authenticator: &core.IamAuthenticator{ApiKey: apiKey},
	})
	if err != nil {
		return "", fmt.Errorf("create IAM policy client: %w", err)
	}

	opts := client.NewListPoliciesOptions(accountID).
		SetType("authorization")

	result, _, err := client.ListPolicies(opts)
	if err != nil {
		return "", fmt.Errorf("list IAM authorization policies: %w", err)
	}

	for _, p := range result.Policies {
		for _, subj := range p.Subjects {
			hasIS, hasImage := false, false
			for _, attr := range subj.Attributes {
				name := core.StringNilMapper(attr.Name)
				switch name {
				case "serviceName":
					if core.StringNilMapper(attr.Value) == "is" {
						hasIS = true
					}
				case "resourceType":
					if core.StringNilMapper(attr.Value) == "image" {
						hasImage = true
					}
				}
			}
			if hasIS && hasImage {
				return core.StringNilMapper(p.ID), nil
			}
		}
	}
	return "", nil
}
