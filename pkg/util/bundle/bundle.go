package bundle

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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

type FilenameInfo struct {
	Preset             string
	Driver             string
	Version            string
	Arch               string
	CustomBundleSuffix string
}

// Bundle name format contains the version number we are managing
// this function will return a description having that versioning info in it
// if bundle name does not mach the default format version will be reported as unknown
func GetDescription(bundleURL string, bundleArch *BundleArch) (*string, error) {
	bundleName, err := GetBundleNameFromURI(bundleURL)
	if err != nil {
		return nil, err
	}
	bundleInfo, err := GetBundleInfoFromName(bundleName)
	if err != nil {
		return nil, err
	}
	description :=
		fmt.Sprintf("%s-%s", bundleDescription, bundleInfo.Version)
	return &description, nil
}

// https://github.com/crc-org/crc/blob/main/pkg/crc/machine/bundle/metadata.go#L263
// GetBundleInfoFromName Parses the bundle filename and returns a FilenameInfo struct
func GetBundleInfoFromName(bundleName string) (*FilenameInfo, error) {
	var filenameInfo FilenameInfo

	/*
		crc_preset_driver_version_arch_customSuffix.crcbundle

		crc                                : Matches the fixed crc part
		(?:(?:_)([[:alpha:]]+))?           : Matches the preset part (optional)
		([[:alpha:]]+)                     : Matches the next mandatory alphabetic part (e.g., libvirt)
		(%s = semverRegex)                 : Matches the version in SemVer format (e.g., 4.16.7 or 4.16.7-ec.2)
		([[:alnum:]]+)                     : Matches the architecture or platform part (e.g. amd64)
		(?:_([0-9]+)(?:_([0-9]+))?)?       : Matches an optional underscore + number (_1234), followed optionally by another underscore + number (_5678)
		\.crcbundle                        : Matches the file extension .crcbundle
	*/
	semverRegex := "(?:0|[1-9]\\d*)\\.(?:0|[1-9]\\d*)\\.(?:0|[1-9]\\d*)(?:-(?:(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\\.(?:0|[1-9]\\d*|\\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\\+(?:[0-9a-zA-Z-]+(?:\\.[0-9a-zA-Z-]+)*))?"
	bundleRegex := `crc(?:(?:_)([[:alpha:]]+))?_([[:alpha:]]+)_(%s)_([[:alnum:]]+)(?:_([0-9]+)(?:_([0-9]+))?)?\.crcbundle`
	compiledRegex := regexp.MustCompile(fmt.Sprintf(bundleRegex, semverRegex))
	filenameParts := compiledRegex.FindStringSubmatch(bundleName)

	if filenameParts == nil {
		return &filenameInfo, fmt.Errorf("bundle filename is in unrecognized format")
	}

	if filenameParts[1] == "" {
		filenameInfo.Preset = "openshift"
	} else {
		filenameInfo.Preset = "microshift"
	}
	filenameInfo.Driver = filenameParts[2]
	filenameInfo.Version = filenameParts[3]
	filenameInfo.Arch = filenameParts[4]
	filenameInfo.CustomBundleSuffix = filenameParts[5]

	return &filenameInfo, nil
}

func GetBundleNameFromURI(bundleURI string) (string, error) {
	switch {
	case strings.HasPrefix(bundleURI, "http://"), strings.HasPrefix(bundleURI, "https://"):
		return path.Base(bundleURI), nil
	case strings.HasPrefix(bundleURI, "file://"):
		return path.Base(bundleURI), nil
	default:
		// local path
		return filepath.Base(bundleURI), nil
	}
}
