package context

import (
	"fmt"
	"os"
	"strings"
)

const (
	originTagName  = "origin"
	originTagValue = "cloud-importer"
)

type ContextArgs struct {
	ProjectName  string
	BackedURL    string
	Debug        bool
	DebugLevel   uint
	KeepState    bool
	ForceDestroy bool
}

type context struct {
	projectName  string
	backedURL    string
	rawScheme    string
	debug        bool
	debugLevel   uint
	keepState    bool
	forceDestroy bool
	tags         map[string]string
}

var c *context

func Init(ca *ContextArgs) {
	url := strings.TrimSuffix(ca.BackedURL, "/")
	rawScheme := extractScheme(url)
	if rawScheme == "cos" {
		url = translateCOSURL(url)
		setupCOSBackendCredentials()
	}
	c = &context{
		projectName:  ca.ProjectName,
		backedURL:    url,
		rawScheme:    rawScheme,
		debug:        ca.Debug,
		debugLevel:   ca.DebugLevel,
		keepState:    ca.KeepState,
		forceDestroy: ca.ForceDestroy,
	}
	addCommonTags()
}

func extractScheme(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		return url[:i]
	}
	return ""
}

// translateCOSURL converts a cos://bucket/path URL to the s3://bucket/path?endpoint=HOST&s3ForcePathStyle=true
// format that Pulumi's S3-compatible backend understands, using the IBM COS regional endpoint.
func translateCOSURL(url string) string {
	region := os.Getenv("IBMCLOUD_REGION")
	if region == "" {
		region = os.Getenv("IC_REGION")
	}
	path := strings.TrimPrefix(url, "cos://")
	var host string
	if ep := os.Getenv("IBMCLOUD_COS_ENDPOINT"); ep != "" {
		host = strings.TrimPrefix(strings.TrimPrefix(ep, "https://"), "http://")
	} else {
		host = fmt.Sprintf("s3.%s.cloud-object-storage.appdomain.cloud", region)
	}
	return fmt.Sprintf("s3://%s?endpoint=%s&s3ForcePathStyle=true", path, host)
}

// setupCOSBackendCredentials maps IBM COS HMAC keys to the standard AWS SDK env vars
// so that Pulumi's S3-compatible backend can authenticate against IBM COS.
func setupCOSBackendCredentials() {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		if v := os.Getenv("IBMCLOUD_COS_ACCESS_KEY"); v != "" {
			os.Setenv("AWS_ACCESS_KEY_ID", v)
		}
	}
	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		if v := os.Getenv("IBMCLOUD_COS_SECRET_KEY"); v != "" {
			os.Setenv("AWS_SECRET_ACCESS_KEY", v)
		}
	}
	if os.Getenv("AWS_DEFAULT_REGION") == "" {
		region := os.Getenv("IBMCLOUD_REGION")
		if region == "" {
			region = os.Getenv("IC_REGION")
		}
		if region != "" {
			os.Setenv("AWS_DEFAULT_REGION", region)
		}
	}
}

// SetTags sets user-provided tags
func SetTags(tags map[string]string) {
	c.tags = tags
	addCommonTags()
}

// GetTagsMap returns tags as a map for standard AWS SDK and Azure
func GetTagsMap() map[string]string {
	return c.tags
}

func ProjectName() string {
	return c.projectName
}

// BackedURL returns the full Pulumi-ready backend URL: the translated base URL
// (cos:// → s3://+endpoint) with the project name inserted before any query string.
func BackedURL() string {
	baseURL := c.backedURL
	if i := strings.IndexByte(baseURL, '?'); i >= 0 {
		return fmt.Sprintf("%s/%s%s", baseURL[:i], c.projectName, baseURL[i:])
	}
	return fmt.Sprintf("%s/%s", baseURL, c.projectName)
}

// RawScheme returns the original URL scheme provided by the user (e.g. "cos", "s3", "gs").
func RawScheme() string {
	return c.rawScheme
}

func Debug() bool {
	return c.debug
}

func DebugLevel() uint {
	return c.debugLevel
}

func KeepState() bool {
	return c.keepState
}

func ForceDestroy() bool {
	return c.forceDestroy
}

func addCommonTags() {
	// Initialize tags if nil
	if c.tags == nil {
		c.tags = make(map[string]string)
	}

	// Add origin tag
	c.tags[originTagName] = originTagValue
}
