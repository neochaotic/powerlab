package encryption

import "testing"

func TestGetMD5ByStr(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"abc", "abc", "900150983cd24fb0d6963f7d28e17f72"},
		{"message digest", "message digest", "f96b697d7cb7938d525a2f31aaf161d0"},
	}
	for _, c := range cases {
		got := GetMD5ByStr(c.in)
		if got != c.want {
			t.Errorf("%s: GetMD5ByStr(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
		if len(got) != 32 {
			t.Errorf("%s: MD5 hex digest must be 32 chars, got %d", c.name, len(got))
		}
	}

	// Determinism: same input always yields the same digest.
	if GetMD5ByStr("repeat") != GetMD5ByStr("repeat") {
		t.Error("GetMD5ByStr is not deterministic")
	}
}
