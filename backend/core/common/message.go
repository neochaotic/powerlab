package common

import (
	"github.com/IceWhaleTech/CasaOS/codegen/message_bus"
)

// Message-bus topic names. PowerLab uses the `powerlab:*` prefix for
// self-describing routing in logs + traces (see Sprint 3 Phase 3 PRs
// #141 and the cloud-drive removal that retired EventCloudFileRecover/
// `casaos:file:recover`).
const (
	EventSystemUtilization = "powerlab:system:utilization"
	EventFileOperate       = "powerlab:file:operate"
)

// EventTypes is the catalog of message-bus topics this service registers
// at startup.
var EventTypes = []message_bus.EventType{
	{Name: EventSystemUtilization, SourceID: SERVICENAME, PropertyTypeList: []message_bus.PropertyType{}},
	{Name: EventFileOperate, SourceID: SERVICENAME, PropertyTypeList: []message_bus.PropertyType{}},
}
