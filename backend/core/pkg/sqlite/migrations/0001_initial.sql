-- +goose Up
-- Initial core schema (Sprint 3 Phase 2.3 — issue #100). Captures
-- verbatim what GORM AutoMigrate produced for AppNotify, SharesDBModel,
-- ConnectionsDBModel, PeerDriveDBModel.
--
-- IF NOT EXISTS keeps this safe for upgrades from installs where
-- AutoMigrate already created these tables.
--
-- Plus the legacy CasaOS table drops that the original code did
-- AFTER AutoMigrate (`o_application`, `o_friend`, `o_person_download`,
-- `o_person_down_record`). Those tables were never present on a fresh
-- install — they're cleanup for upgrades from very old CasaOS revs.

CREATE TABLE IF NOT EXISTS `o_notify` (
    `state` integer,
    `message` text,
    `created_at` text,
    `updated_at` text,
    `id` text,
    `type` integer,
    `icon` text,
    `name` text,
    `class` integer,
    `custom_id` text,
    PRIMARY KEY (`custom_id`)
);

CREATE TABLE IF NOT EXISTS `o_shares` (
    `id` integer,
    `anonymous` numeric,
    `path` text,
    `name` text,
    `updated` integer,
    `created` integer,
    PRIMARY KEY (`id`)
);

CREATE TABLE IF NOT EXISTS `o_connections` (
    `id` integer,
    `updated` integer,
    `created` integer,
    `username` text,
    `password` text,
    `host` text,
    `port` text,
    `status` text,
    `directories` text,
    `mount_point` text,
    PRIMARY KEY (`id`)
);

CREATE TABLE IF NOT EXISTS `peer_drive_db_models` (
    `id` text,
    `updated` integer,
    `created` integer,
    `user_agent` text,
    `display_name` text,
    `device_name` text,
    `model` text,
    `ip` text,
    `os` text,
    `browser` text,
    PRIMARY KEY (`id`)
);

-- Legacy table cleanup (pre-existing GetDb behavior preserved):
DROP TABLE IF EXISTS `o_application`;
DROP TABLE IF EXISTS `o_friend`;
DROP TABLE IF EXISTS `o_person_download`;
DROP TABLE IF EXISTS `o_person_down_record`;

-- +goose Down
DROP TABLE IF EXISTS `peer_drive_db_models`;
DROP TABLE IF EXISTS `o_connections`;
DROP TABLE IF EXISTS `o_shares`;
DROP TABLE IF EXISTS `o_notify`;
