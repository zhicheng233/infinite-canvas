-- +goose Up
CREATE TABLE IF NOT EXISTS tenant_api_configs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    tenant_id BIGINT UNSIGNED NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    api_key VARCHAR(500) NOT NULL,
    models LONGTEXT,
    image_models LONGTEXT,
    video_models LONGTEXT,
    text_models LONGTEXT,
    audio_models LONGTEXT,
    model_routes LONGTEXT,
    model_video_durations LONGTEXT,
    model_video_customizable LONGTEXT,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_tenant_api_configs_tenant_id (tenant_id),
    INDEX idx_tenant_api_configs_deleted_at (deleted_at)
);

INSERT INTO model_config_migrations (
    created_at, updated_at, source, source_id, version, status, target_id, detail
)
SELECT NOW(3), NOW(3), 'tenant_api_config', id, 2, 'pending', 0, '等待应用启动时迁移到规范化模型配置'
FROM tenant_api_configs
ON DUPLICATE KEY UPDATE updated_at = model_config_migrations.updated_at;

-- +goose Down
DELETE operation
FROM channel_model_operations operation
JOIN channel_models channel_model ON channel_model.id = operation.channel_model_id
JOIN model_config_migrations migration ON migration.target_id = channel_model.channel_id
WHERE migration.source = 'tenant_api_config' AND migration.version = 2;

DELETE channel_model
FROM channel_models channel_model
JOIN model_config_migrations migration ON migration.target_id = channel_model.channel_id
WHERE migration.source = 'tenant_api_config' AND migration.version = 2;

DELETE defaults
FROM channel_protocol_defaults defaults
JOIN model_config_migrations migration ON migration.target_id = defaults.channel_id
WHERE migration.source = 'tenant_api_config' AND migration.version = 2;

DELETE channel
FROM channels channel
JOIN model_config_migrations migration ON migration.target_id = channel.id
WHERE migration.source = 'tenant_api_config' AND migration.version = 2;

DELETE FROM model_config_migration_issues
WHERE migration_id IN (
    SELECT id FROM model_config_migrations WHERE source = 'tenant_api_config' AND version = 2
);

DELETE FROM model_config_migrations WHERE source = 'tenant_api_config' AND version = 2;
