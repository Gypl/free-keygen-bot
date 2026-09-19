package installer

import (
	"strings"
	"testing"
)

func TestExtractURI(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		expectedURI string
		expectErr   bool
	}{
		{
			name: "single vless URI with carriage returns and whitespace",
			output: `Some installation logs here...
Generating client configuration...
uri: vless://uuid-1234@192.168.1.1:443?security=reality&sni=example.com#test-node
Installation completed!`,
			expectedURI: "vless://uuid-1234@192.168.1.1:443?security=reality&sni=example.com#test-node",
			expectErr:   false,
		},
		{
			name: "multiple URI lines returns last one",
			output: `Step 1
uri: vless://first@1.1.1.1:443#first
Step 2
uri: trojan://secret@2.2.2.2:443#second
Done.`,
			expectedURI: "trojan://secret@2.2.2.2:443#second",
			expectErr:   false,
		},
		{
			name: "with ANSI escape codes and Windows CRLF",
			output: "\x1b[32mInstallation finished successfully!\x1b[0m\r\n" +
				"uri: ss://YWVzLTEyOC1nY206cGFzc3dvcmQ@1.2.3.4:8388#shadowsocks\r\n" +
				"\x1b[33mHave fun!\x1b[0m\r\n",
			expectedURI: "ss://YWVzLTEyOC1nY206cGFzc3dvcmQ@1.2.3.4:8388#shadowsocks",
			expectErr:   false,
		},
		{
			name: "no uri line present",
			output: `Some error occurred.
Failed to install packages.`,
			expectedURI: "",
			expectErr:   true,
		},
		{
			name: "uri with tabs and spacing",
			output: `Config output:
uri:    vmess://eyJ2IjoiMiIsInBzIjoiIn0=   
All done.`,
			expectedURI: "vmess://eyJ2IjoiMiIsInBzIjoiIn0=",
			expectErr:   false,
		},
		{
			name: "real olcrtc URI with dollar sign in path",
			output: "[OK] Starting olcrtc server...\n" +
				"uri: olcrtc://jitsi?datachannel@room-abc123#e4f5a6b7$4welcome\n" +
				"[OK] Done.",
			expectedURI: "olcrtc://jitsi?datachannel@room-abc123#e4f5a6b7$4welcome",
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, err := ExtractURI(tt.output)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil (uri=%s)", uri)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if strings.TrimSpace(uri) != strings.TrimSpace(tt.expectedURI) {
				t.Errorf("expected %q, got %q", tt.expectedURI, uri)
			}
		})
	}
}
