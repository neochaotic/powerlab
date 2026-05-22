package service

import "testing"

// Locks the install/redeploy contract: a custom app whose compose is
// edited to rename or drop a service must not leave the old service's
// container running. compose up only tears that orphan down when
// RemoveOrphans is set, so the install Up path must always set it.
func TestComposeUpOptions_RemoveOrphans(t *testing.T) {
	opts := composeUpOptions()
	if !opts.Create.RemoveOrphans {
		t.Error("install Up must set Create.RemoveOrphans — a renamed/removed service would otherwise keep its old container running, so two containers answer for one app")
	}
	if !opts.Start.Wait {
		t.Error("install Up should keep Start.Wait so the caller blocks until containers are healthy")
	}
}
