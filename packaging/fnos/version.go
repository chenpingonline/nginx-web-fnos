package fnos

import (
	_ "embed"
	"regexp"
)

// Embed the source manifest so go run and packaged binaries use the same version.
//
//go:embed manifest
var manifest string

var Version = readVersion(manifest)

func readVersion(text string) string {
	matches := regexp.MustCompile(`(?m)^version[ \t]*=[ \t]*([0-9]+\.[0-9]+\.[0-9]+)[ \t]*\r?$`).FindAllStringSubmatch(text, -1)
	if len(matches) != 1 {
		panic("manifest must contain exactly one version in major.minor.patch format")
	}
	return matches[0][1]
}
