package context

import (
	"fmt"

	"github.com/pulumi/pulumi-aws-native/sdk/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
	Tags        map[string]string
}

type context struct {
	projectName string
	backedURL   string
	debug       bool
	debugLevel  uint
	tags        aws.TagArray
}

var c *context

func Init(ca *ContextArgs) {
	c = &context{
		projectName: ca.ProjectName,
		backedURL:   ca.BackedURL,
		debug:       ca.Debug,
		debugLevel:  ca.DebugLevel,
	}
	addCommonTags()
}

func GetTags() aws.TagArray {
	return c.tags
}

func ProjectName() string {
	return c.projectName
}

// Backed url is composed from the base backed url / project name
// this can help us in case we want to automate some destroy only based on
// backed url base....it can check each folder and use it as project name
func BackedURL() string {
	return fmt.Sprintf("%s/%s", c.backedURL, c.projectName)
}

func Debug() bool {
	return c.debug
}

func DebugLevel() uint {
	return c.debugLevel
}

func addCommonTags() {
	c.tags = append(c.tags, aws.TagArgs{
		Key:   pulumi.String(originTagName),
		Value: pulumi.String(originTagValue),
	})
}
