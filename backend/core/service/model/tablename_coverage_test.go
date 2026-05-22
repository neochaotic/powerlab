package model

import "testing"

func TestTableNames(t *testing.T) {
	if got := (&AppNotify{}).TableName(); got != "o_notify" {
		t.Errorf("AppNotify.TableName() = %q, want \"o_notify\"", got)
	}
	if got := (RelyDBModel{}).TableName(); got != "o_rely" {
		t.Errorf("RelyDBModel.TableName() = %q, want \"o_rely\"", got)
	}
}
