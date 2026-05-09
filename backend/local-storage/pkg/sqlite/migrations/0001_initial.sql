-- +goose Up
-- Initial local-storage schema (Sprint 3 Phase 2.4 — issue #100,
-- last service to retire AutoMigrate). Captures verbatim what GORM
-- AutoMigrate produced for model.Merge and model.Volume (table name
-- override "o_disk" preserved for legacy compatibility — see
-- model/o_volume.go::TableName).
--
-- IF NOT EXISTS makes this safe for upgrades from installs where
-- AutoMigrate already created these tables.

CREATE TABLE IF NOT EXISTS `o_disk` (
    `id` integer,
    `uuid` text,
    `mount_point` text,
    `created_at` integer,
    PRIMARY KEY (`id`)
);

CREATE TABLE IF NOT EXISTS `o_merge` (
    `id` integer,
    `fs_type` text,
    `mount_point` text,
    `source_base_path` text,
    `created_at` datetime,
    `updated_at` datetime,
    PRIMARY KEY (`id`)
);

CREATE TABLE IF NOT EXISTS `o_merge_disk` (
    `merge_id` integer,
    `volume_id` integer,
    PRIMARY KEY (`merge_id`, `volume_id`),
    CONSTRAINT `fk_o_merge_disk_merge` FOREIGN KEY (`merge_id`) REFERENCES `o_merge` (`id`),
    CONSTRAINT `fk_o_merge_disk_volume` FOREIGN KEY (`volume_id`) REFERENCES `o_disk` (`id`)
);

-- +goose Down
DROP TABLE IF EXISTS `o_merge_disk`;
DROP TABLE IF EXISTS `o_merge`;
DROP TABLE IF EXISTS `o_disk`;
