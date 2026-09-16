package adb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	binderProxiesReportHeader   = "ACTIVITY MANAGER BINDER PROXY STATE (dumpsys activity binder-proxies)"
	binderProxyInterfacesHeader = "Top proxy interface names held by SYSTEM"
	binderProxyUIDCountsHeader  = "Counts of Binder Proxies held by SYSTEM"
	binderProxyNoPackage        = "NO PACKAGE NAME FOUND"
)

var (
	binderProxyInterfaceCountPattern = regexp.MustCompile(`^#([0-9]+): (.*) x([0-9]+)$`)
	binderProxyUIDCountPattern       = regexp.MustCompile(`^UID[[:space:]]+([0-9]+),[[:space:]]*binder count[[:space:]]*=[[:space:]]*([0-9]+),[[:space:]]*package\(s\)[[:space:]]*=[[:space:]]*(.*)$`)
)

// BinderProxyInterfaceCount is one entry in the ranked list of Binder proxy
// interface descriptors held by system_server.
type BinderProxyInterfaceCount struct {
	Rank          int
	InterfaceName string
	Count         int
}

// BinderProxyUIDCount is the number of Binder proxies held by system_server
// for one UID. PackageNames is empty when Android cannot map the UID to a
// package.
type BinderProxyUIDCount struct {
	UID          int
	BinderCount  int
	PackageNames []string
}

// ActivityBinderProxies is the structured output of
// "dumpsys activity binder-proxies".
//
// Some Android builds omit InterfaceCounts, but UIDCounts is always present in
// a complete report.
type ActivityBinderProxies struct {
	InterfaceCounts []BinderProxyInterfaceCount
	UIDCounts       []BinderProxyUIDCount
}

// ParseActivityBinderProxies parses the output of
// "adb shell dumpsys activity binder-proxies".
//
// The parser is intentionally strict: it returns an error for unknown,
// duplicate, out-of-order, or truncated data instead of silently returning a
// partial report.
func ParseActivityBinderProxies(output []byte) (ActivityBinderProxies, error) {
	var report ActivityBinderProxies
	var seenReportHeader bool
	var seenInterfaceHeader bool
	var seenUIDHeader bool
	interfaceRanks := make(map[int]struct{})
	uids := make(map[int]struct{})

	for index, rawLine := range strings.Split(string(output), "\n") {
		lineNumber := index + 1
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" {
			continue
		}

		if !seenReportHeader {
			if line != binderProxiesReportHeader {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "expected report header, got %q", line)
			}
			seenReportHeader = true
			continue
		}

		switch line {
		case binderProxiesReportHeader:
			return ActivityBinderProxies{}, parseBinderProxiesError(
				lineNumber, "duplicate report header")
		case binderProxyInterfacesHeader:
			if seenInterfaceHeader {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "duplicate interface-count header")
			}
			if seenUIDHeader {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "interface-count section follows UID-count section")
			}
			seenInterfaceHeader = true
			continue
		case binderProxyUIDCountsHeader:
			if seenUIDHeader {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "duplicate UID-count header")
			}
			seenUIDHeader = true
			continue
		}

		if matches := binderProxyInterfaceCountPattern.FindStringSubmatch(line); matches != nil {
			if !seenInterfaceHeader {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "interface count appears before its header")
			}
			if seenUIDHeader {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "interface count appears in UID-count section")
			}

			entry, err := parseBinderProxyInterfaceCount(matches)
			if err != nil {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "%v", err)
			}
			if _, exists := interfaceRanks[entry.Rank]; exists {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "duplicate interface rank %d", entry.Rank)
			}
			expectedRank := len(report.InterfaceCounts) + 1
			if entry.Rank != expectedRank {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "interface rank is %d, expected %d", entry.Rank, expectedRank)
			}
			interfaceRanks[entry.Rank] = struct{}{}
			report.InterfaceCounts = append(report.InterfaceCounts, entry)
			continue
		}

		if matches := binderProxyUIDCountPattern.FindStringSubmatch(line); matches != nil {
			if !seenUIDHeader {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "UID count appears before its header")
			}

			entry, err := parseBinderProxyUIDCount(matches)
			if err != nil {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "%v", err)
			}
			if _, exists := uids[entry.UID]; exists {
				return ActivityBinderProxies{}, parseBinderProxiesError(
					lineNumber, "duplicate UID %d", entry.UID)
			}
			uids[entry.UID] = struct{}{}
			report.UIDCounts = append(report.UIDCounts, entry)
			continue
		}

		return ActivityBinderProxies{}, parseBinderProxiesError(
			lineNumber, "unexpected content %q", line)
	}

	if !seenReportHeader {
		return ActivityBinderProxies{}, fmt.Errorf(
			"parse dumpsys activity binder-proxies: missing report header")
	}
	if !seenUIDHeader {
		return ActivityBinderProxies{}, fmt.Errorf(
			"parse dumpsys activity binder-proxies: missing UID-count section")
	}
	return report, nil
}

func parseBinderProxyInterfaceCount(matches []string) (BinderProxyInterfaceCount, error) {
	rank, err := strconv.Atoi(matches[1])
	if err != nil || rank < 1 {
		return BinderProxyInterfaceCount{}, fmt.Errorf("invalid interface rank %q", matches[1])
	}
	count, err := strconv.Atoi(matches[3])
	if err != nil {
		return BinderProxyInterfaceCount{}, fmt.Errorf("invalid interface count %q", matches[3])
	}
	return BinderProxyInterfaceCount{
		Rank:          rank,
		InterfaceName: matches[2],
		Count:         count,
	}, nil
}

func parseBinderProxyUIDCount(matches []string) (BinderProxyUIDCount, error) {
	uid, err := strconv.Atoi(matches[1])
	if err != nil {
		return BinderProxyUIDCount{}, fmt.Errorf("invalid UID %q", matches[1])
	}
	binderCount, err := strconv.Atoi(matches[2])
	if err != nil {
		return BinderProxyUIDCount{}, fmt.Errorf("invalid binder count %q", matches[2])
	}
	packageNames, err := parseBinderProxyPackageNames(matches[3])
	if err != nil {
		return BinderProxyUIDCount{}, err
	}
	return BinderProxyUIDCount{
		UID:          uid,
		BinderCount:  binderCount,
		PackageNames: packageNames,
	}, nil
}

func parseBinderProxyPackageNames(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == binderProxyNoPackage {
		return nil, nil
	}
	if value == "" {
		return nil, fmt.Errorf("package list is empty")
	}

	parts := strings.Split(value, ";")
	packageNames := make([]string, 0, len(parts))
	for index, part := range parts {
		packageName := strings.TrimSpace(part)
		if packageName == "" {
			if index == len(parts)-1 {
				continue
			}
			return nil, fmt.Errorf("package list contains an empty entry")
		}
		packageNames = append(packageNames, packageName)
	}
	if len(packageNames) == 0 {
		return nil, fmt.Errorf("package list is empty")
	}
	return packageNames, nil
}

func parseBinderProxiesError(lineNumber int, format string, args ...any) error {
	return fmt.Errorf(
		"parse dumpsys activity binder-proxies line %d: %s",
		lineNumber, fmt.Sprintf(format, args...))
}

// GetActivityBinderProxies runs "dumpsys activity binder-proxies" and parses
// its output.
func (d *Device) GetActivityBinderProxies() (ActivityBinderProxies, error) {
	output, err := d.RunCommandTimeout(
		d.CmdTimeoutLong, "dumpsys", "activity", "binder-proxies")
	if err != nil {
		return ActivityBinderProxies{}, fmt.Errorf(
			"run dumpsys activity binder-proxies: %w", err)
	}
	return ParseActivityBinderProxies(output)
}
