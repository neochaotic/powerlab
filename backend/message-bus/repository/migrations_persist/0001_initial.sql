-- +goose Up
-- Initial persist-DB schema for the message-bus service.
-- Captures verbatim what GORM AutoMigrate produced for
-- model.Settings and ysk.YSKCard.
--
-- IF NOT EXISTS keeps this safe for upgrades from installs where
-- AutoMigrate already created these tables.

CREATE TABLE IF NOT EXISTS `settings` (
    `key` text,
    `value` text,
    PRIMARY KEY (`key`)
);

CREATE TABLE IF NOT EXISTS `ysk_cards` (
    `id` text,
    `card_type` text,
    `render_type` text,
    `content` blob,
    `updated` integer,
    `created` integer,
    PRIMARY KEY (`id`)
);

-- +goose Down
DROP TABLE IF EXISTS `ysk_cards`;
DROP TABLE IF EXISTS `settings`;
