// Package model holds shared response envelopes + service-info
// types used across PowerLab backend services. The Result envelope
// here is the canonical "success/message/data" JSON shape every
// V1 endpoint returns.
package model

// Result is the legacy V1 JSON response envelope. Success is an HTTP-
// style status int (200 on happy path, 4xx/5xx on error). Use this
// when adding new V1 endpoints; V2 endpoints generate response shapes
// from the OpenAPI spec via oapi-codegen instead.
type Result struct {
	Success int         `json:"success" example:"200"`
	Message string      `json:"message" example:"ok"`
	Data    interface{} `json:"data" example:"返回结果"`
}
