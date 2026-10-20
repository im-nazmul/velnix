package domain

import "testing"

func TestExactPHPVersions(t *testing.T) {
	for _, v := range []string{"8.3.12", "8.4.0RC1", "8.6.0-dev", "8.3.12+vendor"} {
		if !ValidPHPVersion(v) {
			t.Fatalf("exact version refused: %s", v)
		}
	}
	for _, v := range []string{"", "8", "8.3", "^8.3.12", ">=8.3.12", "8.3.*", "8.3.12\n", "8.3.12;id"} {
		if ValidPHPVersion(v) {
			t.Fatalf("constraint or invalid version accepted: %q", v)
		}
	}
}
