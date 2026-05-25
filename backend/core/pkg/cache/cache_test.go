package cache

import "testing"

// Init returns a usable cache with the core defaults.
func TestInit(t *testing.T) {
	c := Init()
	if c == nil {
		t.Fatal("Init() returned nil")
	}
	c.Set("k", "v", 0)
	if v, ok := c.Get("k"); !ok || v.(string) != "v" {
		t.Errorf("set/get round-trip failed: ok=%v v=%v", ok, v)
	}
}
