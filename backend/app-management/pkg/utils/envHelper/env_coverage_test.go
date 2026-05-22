package envHelper

import "testing"

func TestReplaceDefaultENV_AllKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
		tz   string
		want string
	}{
		{"password", "$DefaultPassword", "", "powerlab"},
		{"username", "$DefaultUserName", "", "admin"},
		{"puid", "$PUID", "", "1000"},
		{"pgid", "$PGID", "", "1000"},
		{"tz echoes argument", "$TZ", "America/Sao_Paulo", "America/Sao_Paulo"},
		{"tz empty argument", "$TZ", "", ""},
		{"unknown key yields empty", "$NotAPlaceholder", "ignored", ""},
		{"empty key yields empty", "", "ignored", ""},
	}
	for _, c := range cases {
		if got := ReplaceDefaultENV(c.key, c.tz); got != c.want {
			t.Errorf("%s: ReplaceDefaultENV(%q, %q) = %q, want %q", c.name, c.key, c.tz, got, c.want)
		}
	}
}

func TestReplaceStringDefaultENV(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"both placeholders", "u=$DefaultUserName p=$DefaultPassword", "u=admin p=powerlab"},
		{"repeated placeholder", "$DefaultPassword:$DefaultPassword", "powerlab:powerlab"},
		{"no placeholder passes through", "plain string", "plain string"},
		{"empty input", "", ""},
		// $TZ/$PUID/$PGID are NOT handled by the string helper — only the
		// user/password placeholders are. Confirm they are left untouched.
		{"tz left untouched", "tz=$TZ", "tz=$TZ"},
	}
	for _, c := range cases {
		if got := ReplaceStringDefaultENV(c.in); got != c.want {
			t.Errorf("%s: ReplaceStringDefaultENV(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// Idempotence: once substituted, re-running must not change the result
// (the literals "admin"/"powerlab" contain no placeholders).
func TestReplaceStringDefaultENV_Idempotent(t *testing.T) {
	once := ReplaceStringDefaultENV("u=$DefaultUserName p=$DefaultPassword")
	twice := ReplaceStringDefaultENV(once)
	if once != twice {
		t.Errorf("not idempotent: once=%q twice=%q", once, twice)
	}
}
