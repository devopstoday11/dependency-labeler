package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/pivotal/deplab/rootfs"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/pivotal/deplab/metadata"
	"github.com/pkg/errors"
)

const DebianPackageListSourceType = "debian_package_list"

func BuildDebianDependencyMetadata(imageName string, rfs rootfs.RootFS) (metadata.Dependency, error) {
	packages, err := getDebianPackages(rfs)

	if len(packages) != 0 {
		sources, _ := getAptSources(rfs)

		sourceMetadata := metadata.DebianPackageListSourceMetadata{
			Packages:   packages,
			AptSources: sources,
		}

		version := Digest(sourceMetadata)

		dpkgList := metadata.Dependency{
			Type: DebianPackageListSourceType,
			Source: metadata.Source{
				Type: "inline",
				Version: map[string]interface{}{
					"sha256": version,
				},
				Metadata: sourceMetadata,
			},
		}

		return dpkgList, nil
	}

	return metadata.Dependency{}, err
}

func Digest(sourceMetadata metadata.DebianPackageListSourceMetadata) string {
	hash := sha256.New()
	encoder := json.NewEncoder(hash)
	_ = encoder.Encode(sourceMetadata)
	version := hex.EncodeToString(hash.Sum(nil))
	return version
}

func getAptSources(rfs rootfs.RootFS) ([]string, error) {
	sources := []string{}
	fileListContent, err := rfs.GetDirContents("/etc/apt/sources.list.d")
	if err != nil {
		// in this case an empty or non-existant directory is not an error
		fileListContent = []string{}
	}

	aFileListContent, err := rfs.GetFileContent("/etc/apt/sources.list")
	if err == nil {
		// in this case an empty or non-existant file is not an error
		fileListContent = append(fileListContent, aFileListContent)
	}

	for _, fileContent := range fileListContent {
		for _, content := range strings.Split(fileContent, "\n") {
			trimmed := strings.TrimSpace(content)
			if !strings.HasPrefix(trimmed, "#") && len(trimmed) != 0 {
				sources = append(sources, content)
			}
		}
	}

	collator := collate.New(language.BritishEnglish)
	sort.Slice(sources, func(i, j int) bool {
		return collator.CompareString(sources[i], sources[j]) < 0
	})

	return sources, nil
}

func getDebianPackages(rfs rootfs.RootFS) ([]metadata.Package, error) {
	var packages []metadata.Package

	statusPackages, err := listPackagesFromStatus(rfs)

	if err != nil {
		return []metadata.Package{}, err
	}

	packages = append(packages, statusPackages...)

	statusDPackages, err := listPackagesFromStatusD(rfs)

	if err != nil {
		return []metadata.Package{}, err
	}

	packages = append(packages, statusDPackages...)

	collator := collate.New(language.BritishEnglish)
	sort.Slice(packages, func(i, j int) bool {
		return collator.CompareString(packages[i].Package, packages[j].Package) < 0
	})

	return packages, nil
}

func ParseStatDBEntry(content string) (metadata.Package, error) {
	pkg := metadata.Package{}

	if strings.TrimSpace(content) == "" {
		return pkg, errors.New("invalid StatDB entry")
	}

	for _, inputLine := range strings.Split(content, "\n") {
		idx := strings.Index(inputLine, ":")
		if idx == -1 {
			continue
		}
		key := inputLine[0:idx]
		value := strings.TrimSpace(inputLine[idx+1:])
		switch key {
		case "Package":
			pkg.Package = value
		case "Version":
			pkg.Version = value
		case "Architecture":
			pkg.Architecture = value
		case "Source":
			idx := strings.Index(value, "(")
			if idx == -1 {
				pkg.Source.Package = strings.TrimSpace(value)
			} else {
				pkg.Source.Package = strings.TrimSpace(value[0:idx])
				version := strings.Trim(value[idx:], " ()")
				pkg.Source.Version = version
				pkg.Source.UpstreamVersion = getUpstreamVersion(version)
			}
		default:
			continue
		}
	}

	if pkg.Source.Package == "" {
		pkg.Source = metadata.PackageSource{
			Package:         pkg.Package,
			Version:         pkg.Version,
			UpstreamVersion: getUpstreamVersion(pkg.Version),
		}
	}
	if pkg.Source.Version == "" {
		pkg.Source.Version = pkg.Version
		pkg.Source.UpstreamVersion = getUpstreamVersion(pkg.Version)
	}

	return pkg, nil
}

func listPackagesFromStatusD(rfs rootfs.RootFS) (packages []metadata.Package, err error) {
	fileList, err := rfs.GetDirContents("/var/lib/dpkg/status.d")
	if err != nil {
		// in this case an empty or non-existant directory is not an error
		fileList = []string{}
	}

	for _, file := range fileList {
		packageEntry, err := ParseStatDBEntry(file)
		if err == nil {
			packages = append(packages, packageEntry)
		}
	}

	return packages, nil
}

func listPackagesFromStatus(rfs rootfs.RootFS) (packages []metadata.Package, err error) {
	statDBString, err := rfs.GetFileContent("/var/lib/dpkg/status")
	if err != nil {
		// in this case an empty or non-existant file is not an error
		statDBString = ""
	}

	statDBEntries := strings.Split(statDBString, "\n\n")
	for _, entryString := range statDBEntries {
		entry, err := ParseStatDBEntry(entryString)
		if err == nil {
			packages = append(packages, entry)
		}
	}

	return packages, nil
}

func getUpstreamVersion(input string) string {
	version := strings.Split(input, "-")[0]
	if strings.Contains(version, ":") {
		version = strings.Split(version, ":")[1]
	}
	return version
}
