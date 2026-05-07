package clipboard

import "testing"

func TestClipboardModeDefaultsToAuto(t *testing.T) {
	t.Setenv(clipboardModeEnv, "")

	if got := clipboardMode(); got != clipboardModeAuto {
		t.Fatalf("clipboardMode() = %q, want %q", got, clipboardModeAuto)
	}
}

func TestClipboardModeHonorsKnownValues(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "native", want: clipboardModeNative},
		{value: "osc52", want: clipboardModeOSC52},
		{value: " OSC52 ", want: clipboardModeOSC52},
		{value: "unknown", want: clipboardModeAuto},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv(clipboardModeEnv, tt.value)
			if got := clipboardMode(); got != tt.want {
				t.Fatalf("clipboardMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSSHSessionPrefersOSC52(t *testing.T) {
	t.Setenv(clipboardModeEnv, "")
	t.Setenv("SSH_CONNECTION", "192.0.2.1 12345 198.51.100.2 22")

	if !shouldPreferOSC52() {
		t.Fatal("shouldPreferOSC52() = false, want true for SSH sessions")
	}
}

func TestNativeModeDoesNotPreferOSC52(t *testing.T) {
	t.Setenv(clipboardModeEnv, clipboardModeNative)
	t.Setenv("SSH_CONNECTION", "192.0.2.1 12345 198.51.100.2 22")

	if shouldPreferOSC52() {
		t.Fatal("shouldPreferOSC52() = true, want false when native mode is forced")
	}
}
