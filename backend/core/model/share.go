package model

// Shares is one SMB share row — id, public-vs-credentialed flag,
// and the on-disk path being shared. Persisted via the casaos
// samba route; mutated by the V1 share endpoints.
type Shares struct {
	ID        uint   `json:"id"`
	Anonymous bool   `json:"anonymous"`
	Path      string `json:"path"`
}
