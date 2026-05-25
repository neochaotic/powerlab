package common_err

import "testing"

// GetMsg returns the message for a known code, and falls back to the
// SERVICE_ERROR message for any code not in the table.
func TestGetMsg_KnownCodesMatchTable(t *testing.T) {
	if len(MsgFlags) == 0 {
		t.Fatal("MsgFlags is empty — nothing to verify")
	}
	for code, want := range MsgFlags {
		if got := GetMsg(code); got != want {
			t.Errorf("GetMsg(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestGetMsg_UnknownCodeFallsBackToServiceError(t *testing.T) {
	const unknown = 99_999_999 // not a defined code
	if _, exists := MsgFlags[unknown]; exists {
		t.Fatalf("test code %d unexpectedly defined in MsgFlags", unknown)
	}
	got := GetMsg(unknown)
	if want := MsgFlags[SERVICE_ERROR]; got != want {
		t.Errorf("GetMsg(unknown) = %q, want SERVICE_ERROR message %q", got, want)
	}
}
