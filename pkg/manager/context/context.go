package context

import (
	"crypto/rand"
	"fmt"

	"github.com/pulumi/pulumi-aws-native/sdk/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	originTagName  = "origin"
	originTagValue = "cloud-importer"
)

type ContextArgs struct {
	BackedURL  string
	Output     string
	Debug      bool
	DebugLevel uint
	Tags       map[string]string
}

type context struct {
	projectName string
	backedURL   string
	output      string
	debug       bool
	debugLevel  uint
	tags        aws.TagArray
}

var c *context

func Init(ca *ContextArgs) {
	c = &context{
		projectName: randomID(),
		backedURL:   ca.BackedURL,
		output:      ca.Output,
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

func BackedURL() string {
	return c.backedURL
}

func Output() string {
	return c.output
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

// // random name for temporary assets required for importing the image
func randomID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("cloud-importer-%x", b)
}
