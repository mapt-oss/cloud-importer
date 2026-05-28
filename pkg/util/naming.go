package util

import (
	"regexp"
	"strings"
)

func sanitize(name string, allowHyphens bool, maxLen int) string {
	name = strings.ToLower(name)
	if allowHyphens {
		name = strings.NewReplacer("_", "-", ".", "-").Replace(name)
		name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "")
	} else {
		name = regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(name, "")
	}
	if len(name) > maxLen {
		name = name[:maxLen]
	}
	return strings.TrimRight(name, "-")
}

// SanitizeStorageAccountName produces a name valid for Azure storage accounts:
// lowercase alphanumeric only (no hyphens), max 24 chars.
func SanitizeStorageAccountName(name string) string {
	return sanitize(name, false, 24)
}

// SanitizeBucketName produces a name valid for S3 and GCS buckets:
// lowercase, hyphens allowed, max 63 chars.
func SanitizeBucketName(name string) string {
	return sanitize(name, true, 63)
}
