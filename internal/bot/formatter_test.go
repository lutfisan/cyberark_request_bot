package bot

import (
	"testing"

	"cybarbot/internal/cyberark"
)

func TestGetRequester(t *testing.T) {
	tests := []struct {
		name     string
		reqor    string
		expected string
	}{
		{"Valid Requestor", "john_doe", "john_doe"},
		{"Empty Requestor", "", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRequester(tt.reqor)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetAccountStr(t *testing.T) {
	tests := []struct {
		name      string
		details   cyberark.AccountDetails
		operation string
		expName   string
		expAddr   string
	}{
		{
			name: "From Details",
			details: cyberark.AccountDetails{
				Properties: cyberark.AccountProperties{
					UserName: "admin",
					Address:  "192.168.1.1",
				},
			},
			operation: "Connect with ignored on ignored",
			expName:   "admin",
			expAddr:   "192.168.1.1",
		},
		{
			name:      "Fallback from Operation string",
			details:   cyberark.AccountDetails{},
			operation: "Connect with root on 10.0.0.5",
			expName:   "root",
			expAddr:   "10.0.0.5",
		},
		{
			name:      "Fallback from Operation string complex",
			details:   cyberark.AccountDetails{},
			operation: "Connect with app_user_1 on server-xyz",
			expName:   "app_user_1",
			expAddr:   "server-xyz",
		},
		{
			name:      "Empty everything",
			details:   cyberark.AccountDetails{},
			operation: "",
			expName:   "Unknown",
			expAddr:   "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotAddr := getAccountStr(tt.details, tt.operation)
			if gotName != tt.expName {
				t.Errorf("expected name %q, got %q", tt.expName, gotName)
			}
			if gotAddr != tt.expAddr {
				t.Errorf("expected address %q, got %q", tt.expAddr, gotAddr)
			}
		})
	}
}

func TestGetTimeFrame(t *testing.T) {
	tests := []struct {
		name       string
		accessFrom int64
		accessTo   int64
		created    int64
		expired    int64
		expected   string
	}{
		{
			name:       "Use AccessFrom and AccessTo",
			accessFrom: 1717280000, // 2024-06-01 22:13:20 UTC
			accessTo:   1717283600, // 2024-06-01 23:13:20 UTC
			created:    0,
			expired:    0,
			expected:   "2024-06-01 22:13 to 2024-06-01 23:13",
		},
		{
			name:       "Fallback to Creation and Expiration",
			accessFrom: 0,
			accessTo:   0,
			created:    1717280000,
			expired:    1717283600,
			expected:   "2024-06-01 22:13 to 2024-06-01 23:13",
		},
		{
			name:       "Zeroes",
			accessFrom: 0,
			accessTo:   0,
			created:    0,
			expired:    0,
			expected:   "Unknown to Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTimeFrame(tt.accessFrom, tt.accessTo, tt.created, tt.expired)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	result := formatTime(1717280000)
	expected := "2024-06-01 22:13 UTC"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	resultUnknown := formatTime(0)
	if resultUnknown != "Unknown" {
		t.Errorf("expected 'Unknown', got %q", resultUnknown)
	}
}
