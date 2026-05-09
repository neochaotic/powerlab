package common

const Version = "0.4.8"

// SERVICENAME is the SourceID this service publishes with every
// message-bus event + registers under in /v2/message_bus/event_type.
// Surfaces in every cross-service log line that mentions a user-service
// event. Sprint 3 Phase 3 rebrand (#106): "CasaOS-UserService" →
// "PowerLab-UserService" so cross-service logs no longer advertise
// upstream CasaOS branding from a PowerLab process.
const SERVICENAME = "PowerLab-UserService"
