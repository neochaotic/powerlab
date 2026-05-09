package common

import (
	"github.com/IceWhaleTech/CasaOS/codegen/message_bus"
)

// Message-bus topic names. Sprint 3 Phase 3 rebrand: prefix migrated
// from `casaos:` to `powerlab:` for self-describing routing in logs +
// traces. No PowerLab component subscribes to the legacy prefix
// (verified by grep across UI + all 6 services), so the rename is
// non-breaking.
//
// EventCloudFileRecover is intentionally kept on the legacy prefix
// until core's parallel cloud-drive infrastructure (drivers/dropbox,
// drivers/google_drive, drivers/onedrive, route/v1/recover.go) is
// removed in a follow-up PR mirroring #139's local-storage cleanup.
// At that point this constant disappears with its callers.
const (
	EventSystemUtilization = "powerlab:system:utilization"
	EventFileOperate       = "powerlab:file:operate"
	EventCloudFileRecover  = "casaos:file:recover"
)

// EventTypes is the catalog of message-bus topics this service registers
// at startup.
var EventTypes = []message_bus.EventType{
	{Name: EventSystemUtilization, SourceID: SERVICENAME, PropertyTypeList: []message_bus.PropertyType{}},
	{Name: EventCloudFileRecover, SourceID: SERVICENAME, PropertyTypeList: []message_bus.PropertyType{}},
	{Name: EventFileOperate, SourceID: SERVICENAME, PropertyTypeList: []message_bus.PropertyType{}},
}
