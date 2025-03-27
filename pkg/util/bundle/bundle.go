package bundle

import (
	"fmt"
	"regexp"
)

const (
	// ImageUsername                   string = "core"
	// ImageInternalKubeconfigFilepath string = "/opt/kubeconfig"

	bundleDescription    = "openshift-local"
	bundleVersionUnknown = "unknown"
)

type BundleArch string

var (
	ARM64 BundleArch = "arm64"
	AMD64 BundleArch = "amd64"
)

var (
	bundleVersionRegex = "\\d.\\d+.\\d+"
)

// Bundle name format contains the version number we are managing
// this function will return a description having that versioning info in it
// if bundle name does not mach the default format version will be reported as unknown
func GetDescription(bundleURL string, bundleArch *BundleArch) (*string, error) {
	var bundleVersion string
	bundleNameRegex := fmt.Sprintf("crc_libvirt_%s_%s.crcbundle", bundleVersionRegex, *bundleArch)
	rn, err := regexp.Compile(bundleNameRegex)
	if err != nil {
		return nil, err
	}
	bundleName := rn.FindString(bundleURL)
	if len(bundleName) > 0 {
		rv, err := regexp.Compile(bundleVersionRegex)
		if err != nil {
			return nil, err
		}
		bundleVersion = rv.FindString(bundleName)
		if len(bundleVersion) == 0 {
			bundleVersion = bundleVersionUnknown
		}
	}
	description :=
		fmt.Sprintf("%s-%s", bundleDescription, bundleVersion)
	return &description, nil
}
