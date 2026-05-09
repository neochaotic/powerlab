-- +goose Up
-- Initial events-DB schema for the message-bus service.
-- Captures verbatim what GORM AutoMigrate produced for
-- model.EventType, model.ActionType, and model.PropertyType
-- (plus the two many2many junction tables).
--
-- IF NOT EXISTS makes this safe on installs where AutoMigrate
-- already created these tables before pkg/migrations was wired in.

CREATE TABLE IF NOT EXISTS `event_types` (
    `source_id` text,
    `name` text,
    PRIMARY KEY (`source_id`, `name`)
);

CREATE TABLE IF NOT EXISTS `action_types` (
    `source_id` text,
    `name` text,
    PRIMARY KEY (`source_id`, `name`)
);

CREATE TABLE IF NOT EXISTS `property_types` (
    `name` text,
    PRIMARY KEY (`name`)
);

CREATE TABLE IF NOT EXISTS `event_type_property_type` (
    `event_type_source_id` text,
    `event_type_name` text,
    `property_type_name` text,
    PRIMARY KEY (`event_type_source_id`, `event_type_name`, `property_type_name`),
    CONSTRAINT `fk_event_type_property_type_event_type` FOREIGN KEY (`event_type_source_id`, `event_type_name`) REFERENCES `event_types` (`source_id`, `name`),
    CONSTRAINT `fk_event_type_property_type_property_type` FOREIGN KEY (`property_type_name`) REFERENCES `property_types` (`name`)
);

CREATE TABLE IF NOT EXISTS `action_type_property_type` (
    `action_type_source_id` text,
    `action_type_name` text,
    `property_type_name` text,
    PRIMARY KEY (`action_type_source_id`, `action_type_name`, `property_type_name`),
    CONSTRAINT `fk_action_type_property_type_action_type` FOREIGN KEY (`action_type_source_id`, `action_type_name`) REFERENCES `action_types` (`source_id`, `name`),
    CONSTRAINT `fk_action_type_property_type_property_type` FOREIGN KEY (`property_type_name`) REFERENCES `property_types` (`name`)
);

-- +goose Down
DROP TABLE IF EXISTS `action_type_property_type`;
DROP TABLE IF EXISTS `event_type_property_type`;
DROP TABLE IF EXISTS `property_types`;
DROP TABLE IF EXISTS `action_types`;
DROP TABLE IF EXISTS `event_types`;
