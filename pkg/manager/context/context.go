package context

import (
	"fmt"
	"strings"
)

const (
	originTagName  = "origin"
	originTagValue = "cloud-importer"
)

type ContextArgs struct {
	ProjectName string
	BackedURL   string
	Debug       bool
	DebugLevel  uint
	KeepState   bool
}

type context struct {
	projectName string
	backedURL   string
	debug       bool
	debugLevel  uint
	keepState   bool
	tags        map[string]string
}

var c *context

func Init(ca *ContextArgs) {
	c = &context{
		projectName: ca.ProjectName,
		backedURL:   ca.BackedURL,
		debug:       ca.Debug,
		debugLevel:  ca.DebugLevel,
		keepState:   ca.KeepState,
	}
	addCommonTags()
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

// Backed url is composed from the base backed url / project name
// this can help us in case we want to automate some destroy only based on
// backed url base....it can check each folder and use it as project name
func BackedURL() string {
	// Remove trailing slash from backedURL to avoid Pulumi crashes
	baseURL := strings.TrimSuffix(c.backedURL, "/")
	return fmt.Sprintf("%s/%s", baseURL, c.projectName)
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

func addCommonTags() {
	// Initialize tags if nil
	if c.tags == nil {
		c.tags = make(map[string]string)
	}

	// Add origin tag
	c.tags[originTagName] = originTagValue
}
