package adb

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/prife/goadb/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseActivityBinderProxies(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   ActivityBinderProxies
	}{
		{
			name: "AOSP report with interface counts",
			output: `ACTIVITY MANAGER BINDER PROXY STATE (dumpsys activity binder-proxies)
Top proxy interface names held by SYSTEM
    #1: android.content.IIntentReceiver x552
    #2: <cleared weak-ref> x438
    #3:  x20
    #4: <proxy to dead node> x1
  Counts of Binder Proxies held by SYSTEM
    UID 1000, binder count = 800, package(s)= android; com.android.settings;
    UID 1041, binder count = 3, package(s)= NO PACKAGE NAME FOUND
`,
			want: ActivityBinderProxies{
				InterfaceCounts: []BinderProxyInterfaceCount{
					{Rank: 1, InterfaceName: "android.content.IIntentReceiver", Count: 552},
					{Rank: 2, InterfaceName: "<cleared weak-ref>", Count: 438},
					{Rank: 3, InterfaceName: "", Count: 20},
					{Rank: 4, InterfaceName: "<proxy to dead node>", Count: 1},
				},
				UIDCounts: []BinderProxyUIDCount{
					{UID: 1000, BinderCount: 800, PackageNames: []string{"android", "com.android.settings"}},
					{UID: 1041, BinderCount: 3},
				},
			},
		},
		{
			name: "vendor report without interface counts",
			output: "\r\nACTIVITY MANAGER BINDER PROXY STATE (dumpsys activity binder-proxies)\r\n" +
				"  Counts of Binder Proxies held by SYSTEM\r\n" +
				"    UID 1001, binder count = 260, package(s)= com.android.phone; com.android.stk;\r\n" +
				"    UID 1017, binder count = 8, package(s)= NO PACKAGE NAME FOUND",
			want: ActivityBinderProxies{
				UIDCounts: []BinderProxyUIDCount{
					{UID: 1001, BinderCount: 260, PackageNames: []string{"com.android.phone", "com.android.stk"}},
					{UID: 1017, BinderCount: 8},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseActivityBinderProxies([]byte(tt.output))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseActivityBinderProxiesRejectsInvalidReports(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr string
	}{
		{
			name:    "empty output",
			wantErr: "missing report header",
		},
		{
			name:    "permission denial",
			output:  "Permission Denial: can't dump ActivityManager",
			wantErr: "line 1: expected report header",
		},
		{
			name:    "missing UID section",
			output:  binderProxiesReportHeader + "\n" + binderProxyInterfacesHeader,
			wantErr: "missing UID-count section",
		},
		{
			name: "interface before header",
			output: binderProxiesReportHeader + "\n" +
				"#1: android.os.IMessenger x2\n" + binderProxyUIDCountsHeader,
			wantErr: "line 2: interface count appears before its header",
		},
		{
			name: "UID before header",
			output: binderProxiesReportHeader + "\n" +
				"UID 1000, binder count = 2, package(s)= android;",
			wantErr: "line 2: UID count appears before its header",
		},
		{
			name: "duplicate UID",
			output: binderProxiesReportHeader + "\n" + binderProxyUIDCountsHeader + "\n" +
				"UID 1000, binder count = 2, package(s)= android;\n" +
				"UID 1000, binder count = 3, package(s)= android;",
			wantErr: "line 4: duplicate UID 1000",
		},
		{
			name: "non-contiguous interface rank",
			output: binderProxiesReportHeader + "\n" + binderProxyInterfacesHeader + "\n" +
				"#2: android.os.IMessenger x2\n" + binderProxyUIDCountsHeader,
			wantErr: "line 3: interface rank is 2, expected 1",
		},
		{
			name: "empty package entry",
			output: binderProxiesReportHeader + "\n" + binderProxyUIDCountsHeader + "\n" +
				"UID 1000, binder count = 2, package(s)= android;; com.android.settings;",
			wantErr: "line 3: package list contains an empty entry",
		},
		{
			name:    "unknown line",
			output:  binderProxiesReportHeader + "\n" + binderProxyUIDCountsHeader + "\nwat",
			wantErr: "line 3: unexpected content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseActivityBinderProxies([]byte(tt.output))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Equal(t, ActivityBinderProxies{}, got)
		})
	}
}

func TestDeviceGetActivityBinderProxies(t *testing.T) {
	output := binderProxiesReportHeader + "\n" + binderProxyUIDCountsHeader + "\n" +
		"UID 1000, binder count = 2, package(s)= android;\n"
	server := &MockServer{
		Status: wire.StatusSuccess,
		mockConn: mockConn{
			Buffer: bytes.NewBufferString(output),
		},
	}
	device := (&Adb{server}).Device(AnyDevice())

	report, err := device.GetActivityBinderProxies()
	require.NoError(t, err)
	require.Len(t, server.Requests, 2)
	assert.Equal(t, "host:transport-any", server.Requests[0])
	assert.Equal(t, "shell:dumpsys activity binder-proxies", server.Requests[1])
	assert.Equal(t, []BinderProxyUIDCount{
		{UID: 1000, BinderCount: 2, PackageNames: []string{"android"}},
	}, report.UIDCounts)
}

func TestDeviceGetActivityBinderProxiesLive(t *testing.T) {
	serialList := os.Getenv("GOADB_TEST_DEVICES")
	if serialList == "" {
		t.Skip("set GOADB_TEST_DEVICES to a comma-separated device serial list")
	}

	client, err := New()
	require.NoError(t, err)
	for _, serial := range strings.Split(serialList, ",") {
		serial := strings.TrimSpace(serial)
		if serial == "" {
			continue
		}
		t.Run(serial, func(t *testing.T) {
			report, err := client.Device(DeviceWithSerial(serial)).GetActivityBinderProxies()
			require.NoError(t, err)
			require.NotEmpty(t, report.UIDCounts)

			seenUIDs := make(map[int]struct{}, len(report.UIDCounts))
			totalBinderProxies := 0
			packageNames := 0
			for _, entry := range report.UIDCounts {
				assert.GreaterOrEqual(t, entry.UID, 0)
				assert.Greater(t, entry.BinderCount, 0)
				_, duplicate := seenUIDs[entry.UID]
				assert.False(t, duplicate, "duplicate UID %d", entry.UID)
				seenUIDs[entry.UID] = struct{}{}
				totalBinderProxies += entry.BinderCount
				packageNames += len(entry.PackageNames)
				for _, packageName := range entry.PackageNames {
					assert.NotEmpty(t, packageName)
				}
			}

			for index, entry := range report.InterfaceCounts {
				assert.Equal(t, index+1, entry.Rank)
				assert.Greater(t, entry.Count, 0)
				if index > 0 {
					assert.GreaterOrEqual(t, report.InterfaceCounts[index-1].Count, entry.Count)
				}
			}
			t.Logf(
				"parsed %d interface rows, %d UID rows, %d package names, and %d Binder proxies",
				len(report.InterfaceCounts), len(report.UIDCounts), packageNames, totalBinderProxies)
		})
	}
}
