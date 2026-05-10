package model

// PageReq is the standard pagination request envelope used across
// list endpoints. Index is 1-based; Size defaults to a "huge"
// number when unset so callers that don't paginate get every row.
type PageReq struct {
	Index int `json:"page" form:"index"`
	Size  int `json:"size" form:"size"`
}

// Numeric bounds for unbounded "all rows" pagination defaults.
const MaxUint = ^uint(0)
const MinUint = 0
const MaxInt = int(MaxUint >> 1)
const MinInt = -MaxInt - 1

// Validate clamps Index ≥ 1 and Size ≥ 1, defaulting Size to
// 100000 (effectively "all") when unset. Mutates the receiver.
func (p *PageReq) Validate() {
	if p.Index < 1 {
		p.Index = 1
	}
	if p.Size < 1 {
		p.Size = 100000
	}
	// if p.PerPage < 1 {
	// 	p.PerPage = MaxInt
	// }
}
