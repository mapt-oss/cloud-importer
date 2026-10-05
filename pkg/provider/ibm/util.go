package ibm

import (
	"regexp"
	"strings"

	"github.com/mapt-oss/cloud-importer/pkg/util"
)

// sanitizeImageName produces a name valid for IBM VPC custom images:
// lowercase letters, digits, and hyphens; must start with a letter; max 63 chars.
// IBM VPC requires the name to start with a letter, so we prepend "img-" when
// the sanitized result starts with a digit or hyphen.
func sanitizeImageName(name string) string {
	name = strings.ToLower(name)
	name = strings.NewReplacer("_", "-", ".", "-").Replace(name)
	name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "")
	name = strings.TrimRight(name, "-")
	if len(name) == 0 {
		return "img"
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "img-" + name
	}
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	return name
}

// stableBucketName derives a deterministic IBM COS bucket name from the image
// name so that retries with the same --image-name reuse the existing bucket.
// The "tmp-" prefix marks it as ephemeral. Bucket names follow the same
// rules as S3/GCS: lowercase, hyphens, 3-63 chars.
func stableBucketName(imageName string) string {
	name := "tmp-" + util.SanitizeBucketName(imageName)
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.TrimRight(name, "-")
}
