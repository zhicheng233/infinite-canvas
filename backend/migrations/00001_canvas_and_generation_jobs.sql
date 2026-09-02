-- +goose Up
CREATE TABLE IF NOT EXISTS canvas_projects (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    tenant_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    project_id VARCHAR(64) NOT NULL,
    schema_version BIGINT NOT NULL DEFAULT 2,
    revision BIGINT UNSIGNED NOT NULL DEFAULT 1,
    title VARCHAR(200) NOT NULL,
    nodes LONGTEXT,
    connections LONGTEXT,
    chat_sessions LONGTEXT,
    active_chat_id VARCHAR(64),
    background_mode VARCHAR(20) DEFAULT 'lines',
    show_image_info BOOLEAN DEFAULT FALSE,
    viewport_x DOUBLE DEFAULT 0,
    viewport_y DOUBLE DEFAULT 0,
    viewport_k DOUBLE DEFAULT 1,
    PRIMARY KEY (id),
    INDEX idx_canvas_projects_deleted_at (deleted_at),
    INDEX idx_canvas_projects_tenant_id (tenant_id),
    INDEX idx_canvas_projects_user_id (user_id),
    UNIQUE INDEX idx_canvas_project_identity (tenant_id, user_id, project_id)
);

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'canvas_projects' AND column_name = 'schema_version') = 0,
    'ALTER TABLE canvas_projects ADD COLUMN schema_version BIGINT NOT NULL DEFAULT 2 AFTER project_id',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'canvas_projects' AND column_name = 'revision') = 0,
    'ALTER TABLE canvas_projects ADD COLUMN revision BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER schema_version',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'canvas_projects' AND index_name = 'idx_canvas_projects_project_id') > 0,
    'ALTER TABLE canvas_projects DROP INDEX idx_canvas_projects_project_id',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

SET @sql = IF(
    (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'canvas_projects' AND index_name = 'idx_canvas_project_identity') = 0,
    'ALTER TABLE canvas_projects ADD UNIQUE INDEX idx_canvas_project_identity (tenant_id, user_id, project_id)',
    'SELECT 1'
);
PREPARE statement FROM @sql;
EXECUTE statement;
DEALLOCATE PREPARE statement;

CREATE TABLE IF NOT EXISTS generation_jobs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    request_id VARCHAR(64) NOT NULL,
    tenant_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    capability VARCHAR(20) NOT NULL,
    model_name VARCHAR(191) NOT NULL,
    channel_id BIGINT UNSIGNED NOT NULL,
    channel_model_id BIGINT UNSIGNED NOT NULL,
    channel_name VARCHAR(100),
    channel_base_url VARCHAR(500),
    video_route VARCHAR(50),
    billing_amount BIGINT NOT NULL,
    spend_transaction_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    refund_transaction_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    upstream_task_id VARCHAR(191),
    status VARCHAR(20) NOT NULL,
    failure_reason VARCHAR(500),
    PRIMARY KEY (id),
    UNIQUE INDEX idx_generation_jobs_request_id (request_id),
    INDEX idx_generation_jobs_deleted_at (deleted_at),
    INDEX idx_generation_jobs_tenant_id (tenant_id),
    INDEX idx_generation_jobs_user_id (user_id),
    INDEX idx_generation_jobs_capability (capability),
    INDEX idx_generation_jobs_channel_id (channel_id),
    INDEX idx_generation_jobs_channel_model_id (channel_model_id),
    INDEX idx_generation_jobs_spend_transaction_id (spend_transaction_id),
    INDEX idx_generation_jobs_refund_transaction_id (refund_transaction_id),
    INDEX idx_generation_jobs_upstream_task_id (upstream_task_id),
    INDEX idx_generation_jobs_status (status)
);

-- +goose Down
DROP TABLE IF EXISTS generation_jobs;
