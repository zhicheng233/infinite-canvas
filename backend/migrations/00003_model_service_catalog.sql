-- +goose Up
CREATE TABLE IF NOT EXISTS channels (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    name VARCHAR(100) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    api_key VARCHAR(500) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    video_api_standard VARCHAR(20) NOT NULL DEFAULT 'default',
    config_revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    INDEX idx_channels_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS channel_models (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    channel_id BIGINT UNSIGNED NOT NULL,
    model_name VARCHAR(191) NOT NULL,
    catalog_model_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    upstream_model_id VARCHAR(191) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    discovery_status VARCHAR(20) NOT NULL DEFAULT 'present',
    last_discovered_at DATETIME(3) NULL,
    config_revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    legacy_unreviewed BOOLEAN NOT NULL DEFAULT FALSE,
    capabilities VARCHAR(100),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    image_generate_route VARCHAR(30),
    image_edit_route VARCHAR(30),
    video_route VARCHAR(30),
    video_durations VARCHAR(200),
    video_customizable BOOLEAN NOT NULL DEFAULT FALSE,
    video_custom_config TEXT,
    sort_order BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_channel_model (channel_id, model_name),
    INDEX idx_channel_models_catalog_model_id (catalog_model_id),
    INDEX idx_channel_models_status (status)
);

CREATE TABLE IF NOT EXISTS credit_pricing (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    channel_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    tenant_id BIGINT UNSIGNED NOT NULL,
    model VARCHAR(100) NOT NULL,
    credits_per_unit BIGINT NOT NULL,
    unit_type VARCHAR(20) NOT NULL DEFAULT 'per_image',
    pricing_mode VARCHAR(30) NOT NULL DEFAULT 'per_unit',
    pricing_rule LONGTEXT,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_tenant_model_channel (tenant_id, model, channel_id)
);

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channels' AND column_name = 'config_revision') = 0,
    'ALTER TABLE channels ADD COLUMN config_revision BIGINT UNSIGNED NOT NULL DEFAULT 1',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channel_models' AND column_name = 'catalog_model_id') = 0,
    'ALTER TABLE channel_models ADD COLUMN catalog_model_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER model_name',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channel_models' AND column_name = 'upstream_model_id') = 0,
    'ALTER TABLE channel_models ADD COLUMN upstream_model_id VARCHAR(200) NOT NULL DEFAULT '''' AFTER catalog_model_id',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channel_models' AND column_name = 'status') = 0,
    'ALTER TABLE channel_models ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT ''draft'' AFTER upstream_model_id',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channel_models' AND column_name = 'discovery_status') = 0,
    'ALTER TABLE channel_models ADD COLUMN discovery_status VARCHAR(20) NOT NULL DEFAULT ''present'' AFTER status',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channel_models' AND column_name = 'last_discovered_at') = 0,
    'ALTER TABLE channel_models ADD COLUMN last_discovered_at DATETIME(3) NULL AFTER discovery_status',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channel_models' AND column_name = 'config_revision') = 0,
    'ALTER TABLE channel_models ADD COLUMN config_revision BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER last_discovered_at',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'channel_models' AND column_name = 'legacy_unreviewed') = 0,
    'ALTER TABLE channel_models ADD COLUMN legacy_unreviewed BOOLEAN NOT NULL DEFAULT FALSE AFTER config_revision',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

CREATE TABLE IF NOT EXISTS catalog_models (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    public_key VARCHAR(191) NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_catalog_models_public_key (public_key),
    INDEX idx_catalog_models_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS channel_protocol_defaults (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    channel_id BIGINT UNSIGNED NOT NULL,
    capability VARCHAR(20) NOT NULL,
    operation VARCHAR(30) NOT NULL,
    adapter VARCHAR(50) NOT NULL,
    config_json LONGTEXT,
    config_version BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_channel_protocol_default (channel_id, capability, operation),
    INDEX idx_channel_protocol_defaults_deleted_at (deleted_at),
    INDEX idx_channel_protocol_defaults_channel_id (channel_id)
);

CREATE TABLE IF NOT EXISTS channel_model_operations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    channel_model_id BIGINT UNSIGNED NOT NULL,
    capability VARCHAR(20) NOT NULL,
    operation VARCHAR(30) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    protocol_mode VARCHAR(20) NOT NULL,
    adapter VARCHAR(50),
    config_json LONGTEXT,
    config_version BIGINT NOT NULL DEFAULT 1,
    contract_key VARCHAR(64),
    PRIMARY KEY (id),
    UNIQUE INDEX idx_channel_model_operation (channel_model_id, capability, operation),
    INDEX idx_channel_model_operations_deleted_at (deleted_at),
    INDEX idx_channel_model_operations_channel_model_id (channel_model_id),
    INDEX idx_channel_model_operations_capability (capability),
    INDEX idx_channel_model_operations_enabled (enabled),
    INDEX idx_channel_model_operations_contract_key (contract_key)
);

CREATE TABLE IF NOT EXISTS model_pricing_rules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    tenant_id BIGINT UNSIGNED NOT NULL,
    catalog_model_id BIGINT UNSIGNED NOT NULL,
    capability VARCHAR(20) NOT NULL,
    scope VARCHAR(20) NOT NULL,
    scope_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    credits_per_unit BIGINT NOT NULL,
    unit_type VARCHAR(20) NOT NULL,
    pricing_mode VARCHAR(30) NOT NULL,
    pricing_rule LONGTEXT,
    config_revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_model_pricing_rule (tenant_id, catalog_model_id, capability, scope, scope_id),
    INDEX idx_model_pricing_rules_deleted_at (deleted_at),
    INDEX idx_model_pricing_rules_tenant_id (tenant_id),
    INDEX idx_model_pricing_rules_catalog_model_id (catalog_model_id)
);

CREATE TABLE IF NOT EXISTS model_config_audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    tenant_id BIGINT UNSIGNED NOT NULL,
    actor_user_id BIGINT UNSIGNED NOT NULL,
    resource VARCHAR(50) NOT NULL,
    resource_id BIGINT UNSIGNED NOT NULL,
    action VARCHAR(30) NOT NULL,
    before_json LONGTEXT,
    after_json LONGTEXT,
    PRIMARY KEY (id),
    INDEX idx_model_config_audit_logs_deleted_at (deleted_at),
    INDEX idx_model_config_audit_logs_tenant_id (tenant_id),
    INDEX idx_model_config_audit_logs_actor_user_id (actor_user_id),
    INDEX idx_model_config_audit_logs_resource (resource),
    INDEX idx_model_config_audit_logs_resource_id (resource_id),
    INDEX idx_model_config_audit_logs_action (action)
);

CREATE TABLE IF NOT EXISTS model_config_migrations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    source VARCHAR(50) NOT NULL,
    source_id BIGINT UNSIGNED NOT NULL,
    version BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    target_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    detail LONGTEXT,
    completed_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_model_config_migration (source, source_id, version),
    INDEX idx_model_config_migrations_deleted_at (deleted_at),
    INDEX idx_model_config_migrations_status (status),
    INDEX idx_model_config_migrations_target_id (target_id)
);

CREATE TABLE IF NOT EXISTS model_config_migration_issues (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    migration_id BIGINT UNSIGNED NOT NULL,
    resource VARCHAR(50) NOT NULL,
    identifier VARCHAR(250) NOT NULL,
    reason VARCHAR(500) NOT NULL,
    payload_json LONGTEXT,
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (id),
    INDEX idx_model_config_migration_issues_deleted_at (deleted_at),
    INDEX idx_model_config_migration_issues_migration_id (migration_id),
    INDEX idx_model_config_migration_issues_resource (resource),
    INDEX idx_model_config_migration_issues_resolved (resolved)
);

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'auto_routing_pools' AND column_name = 'catalog_model_id') = 0,
    'ALTER TABLE auto_routing_pools ADD COLUMN catalog_model_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER id, ADD INDEX idx_auto_routing_pools_catalog_model_id (catalog_model_id)',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

INSERT INTO catalog_models (created_at, updated_at, public_key, display_name)
SELECT NOW(3), NOW(3), TRIM(model_name), TRIM(model_name)
FROM channel_models
WHERE TRIM(model_name) <> ''
GROUP BY TRIM(model_name)
ON DUPLICATE KEY UPDATE updated_at = updated_at;

UPDATE channel_models cm
JOIN catalog_models catalog ON catalog.public_key = TRIM(cm.model_name)
SET cm.catalog_model_id = catalog.id,
    cm.upstream_model_id = IF(TRIM(cm.upstream_model_id) = '', TRIM(cm.model_name), cm.upstream_model_id),
    cm.status = IF(cm.enabled, 'active', 'disabled'),
    cm.discovery_status = 'present',
    cm.last_discovered_at = COALESCE(cm.last_discovered_at, cm.updated_at, NOW(3)),
    cm.config_revision = IF(cm.config_revision = 0, 1, cm.config_revision),
    cm.legacy_unreviewed = TRUE;

INSERT IGNORE INTO channel_protocol_defaults (created_at, updated_at, channel_id, capability, operation, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'image', 'generate', 'auto', '{}', 1 FROM channels;
INSERT IGNORE INTO channel_protocol_defaults (created_at, updated_at, channel_id, capability, operation, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'image', 'edit', 'auto', '{}', 1 FROM channels;
INSERT IGNORE INTO channel_protocol_defaults (created_at, updated_at, channel_id, capability, operation, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'video', 'generate', IF(video_api_standard = 'binghuo', 'binghuo', 'auto'), '{}', 1 FROM channels;
INSERT IGNORE INTO channel_protocol_defaults (created_at, updated_at, channel_id, capability, operation, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'text', 'generate', 'openai', '{}', 1 FROM channels;
INSERT IGNORE INTO channel_protocol_defaults (created_at, updated_at, channel_id, capability, operation, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'audio', 'generate', 'openai', '{}', 1 FROM channels;

INSERT IGNORE INTO channel_model_operations (created_at, updated_at, channel_model_id, capability, operation, enabled, protocol_mode, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'image', 'generate', TRUE,
       IF(COALESCE(image_generate_route, '', 'auto') IN ('', 'auto'), 'inherit', 'override'),
       IF(COALESCE(image_generate_route, '') IN ('', 'auto'), '', image_generate_route), '{}', 1
FROM channel_models WHERE JSON_VALID(capabilities) AND JSON_CONTAINS(capabilities, '"image"');

INSERT IGNORE INTO channel_model_operations (created_at, updated_at, channel_model_id, capability, operation, enabled, protocol_mode, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'image', 'edit', TRUE,
       IF(COALESCE(image_edit_route, '') IN ('', 'auto'), 'inherit', 'override'),
       IF(COALESCE(image_edit_route, '') IN ('', 'auto'), '', image_edit_route), '{}', 1
FROM channel_models WHERE JSON_VALID(capabilities) AND JSON_CONTAINS(capabilities, '"image"');

INSERT IGNORE INTO channel_model_operations (created_at, updated_at, channel_model_id, capability, operation, enabled, protocol_mode, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), cm.id, 'video', 'generate', TRUE,
       IF(c.video_api_standard = 'binghuo' OR COALESCE(cm.video_route, '') IN ('', 'auto'), 'inherit', 'override'),
       IF(c.video_api_standard = 'binghuo' OR COALESCE(cm.video_route, '') IN ('', 'auto'), '', cm.video_route),
       JSON_OBJECT('durations', IF(JSON_VALID(cm.video_durations), CAST(cm.video_durations AS JSON), JSON_ARRAY()), 'customizable', cm.video_customizable, 'custom_config', IF(JSON_VALID(cm.video_custom_config), CAST(cm.video_custom_config AS JSON), JSON_OBJECT()), 'legacy_shadow_route', IF(c.video_api_standard = 'binghuo' AND COALESCE(cm.video_route, '') NOT IN ('', 'auto'), cm.video_route, '')), 1
FROM channel_models cm JOIN channels c ON c.id = cm.channel_id
WHERE JSON_VALID(cm.capabilities) AND JSON_CONTAINS(cm.capabilities, '"video"');

INSERT IGNORE INTO channel_model_operations (created_at, updated_at, channel_model_id, capability, operation, enabled, protocol_mode, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'text', 'generate', TRUE, 'inherit', '', '{}', 1
FROM channel_models WHERE JSON_VALID(capabilities) AND JSON_CONTAINS(capabilities, '"text"');

INSERT IGNORE INTO channel_model_operations (created_at, updated_at, channel_model_id, capability, operation, enabled, protocol_mode, adapter, config_json, config_version)
SELECT NOW(3), NOW(3), id, 'audio', 'generate', TRUE, 'inherit', '', '{}', 1
FROM channel_models WHERE JSON_VALID(capabilities) AND JSON_CONTAINS(capabilities, '"audio"');

INSERT IGNORE INTO model_pricing_rules (created_at, updated_at, tenant_id, catalog_model_id, capability, scope, scope_id, credits_per_unit, unit_type, pricing_mode, pricing_rule, config_revision)
SELECT NOW(3), NOW(3), p.tenant_id, catalog.id,
       CASE WHEN p.pricing_mode = 'video_dynamic' OR p.unit_type IN ('per_video', 'per_video_second') THEN 'video'
            WHEN p.unit_type = 'per_image' THEN 'image'
            ELSE 'text' END,
       IF(p.channel_id = 0, 'default', 'implementation'),
       IF(p.channel_id = 0, 0, COALESCE(cm.id, 0)),
       p.credits_per_unit, p.unit_type, p.pricing_mode, p.pricing_rule, 1
FROM credit_pricing p
JOIN catalog_models catalog ON catalog.public_key = p.model
LEFT JOIN channel_models cm ON cm.channel_id = p.channel_id AND cm.model_name = p.model AND cm.deleted_at IS NULL
WHERE p.channel_id = 0 OR cm.id IS NOT NULL;

UPDATE auto_routing_pools pool
JOIN catalog_models catalog ON catalog.public_key = pool.public_model_name
SET pool.catalog_model_id = catalog.id
WHERE pool.catalog_model_id = 0;

-- +goose Down
ALTER TABLE auto_routing_pools DROP INDEX idx_auto_routing_pools_catalog_model_id, DROP COLUMN catalog_model_id;
DROP TABLE IF EXISTS model_config_migration_issues;
DROP TABLE IF EXISTS model_config_migrations;
DROP TABLE IF EXISTS model_config_audit_logs;
DROP TABLE IF EXISTS model_pricing_rules;
DROP TABLE IF EXISTS channel_model_operations;
DROP TABLE IF EXISTS channel_protocol_defaults;
DROP TABLE IF EXISTS catalog_models;
ALTER TABLE channel_models
    DROP COLUMN legacy_unreviewed,
    DROP COLUMN config_revision,
    DROP COLUMN last_discovered_at,
    DROP COLUMN discovery_status,
    DROP COLUMN status,
    DROP COLUMN upstream_model_id,
    DROP COLUMN catalog_model_id;
ALTER TABLE channels DROP COLUMN config_revision;
