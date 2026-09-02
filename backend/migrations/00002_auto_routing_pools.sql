-- +goose Up
CREATE TABLE auto_routing_pools (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    public_model_name VARCHAR(200) NOT NULL,
    capability VARCHAR(20) NOT NULL,
    contract_key VARCHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    max_attempts BIGINT NOT NULL DEFAULT 2,
    PRIMARY KEY (id),
    INDEX idx_auto_routing_pools_deleted_at (deleted_at),
    INDEX idx_auto_routing_pools_enabled (enabled),
    UNIQUE INDEX idx_auto_pool_model_capability (public_model_name, capability)
);

CREATE TABLE auto_routing_pool_members (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    pool_id BIGINT UNSIGNED NOT NULL,
    channel_model_id BIGINT UNSIGNED NOT NULL,
    priority BIGINT NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (id),
    INDEX idx_auto_routing_pool_members_deleted_at (deleted_at),
    INDEX idx_auto_routing_pool_members_pool_id (pool_id),
    INDEX idx_auto_routing_pool_members_channel_model_id (channel_model_id),
    INDEX idx_auto_routing_pool_members_enabled (enabled),
    UNIQUE INDEX idx_auto_pool_member (pool_id, channel_model_id)
);

CREATE TABLE generation_attempts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    request_id VARCHAR(64) NOT NULL,
    attempt_no BIGINT NOT NULL,
    pool_id BIGINT UNSIGNED NOT NULL,
    channel_id BIGINT UNSIGNED NOT NULL,
    channel_model_id BIGINT UNSIGNED NOT NULL,
    status_code BIGINT NOT NULL DEFAULT 0,
    response_time_ms BIGINT NOT NULL DEFAULT 0,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    failure_category VARCHAR(30),
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    counts_for_health BOOLEAN NOT NULL DEFAULT FALSE,
    error_message VARCHAR(500),
    PRIMARY KEY (id),
    INDEX idx_generation_attempts_deleted_at (deleted_at),
    INDEX idx_generation_attempts_request_id (request_id),
    UNIQUE INDEX idx_generation_attempt_request (request_id, attempt_no),
    INDEX idx_generation_attempts_pool_id (pool_id),
    INDEX idx_generation_attempts_channel_id (channel_id),
    INDEX idx_generation_attempts_channel_model_id (channel_model_id),
    INDEX idx_generation_attempts_status_code (status_code),
    INDEX idx_generation_attempts_success (success),
    INDEX idx_generation_attempts_failure_category (failure_category),
    INDEX idx_generation_attempts_retryable (retryable),
    INDEX idx_generation_attempts_counts_for_health (counts_for_health)
);

ALTER TABLE generation_jobs
    ADD COLUMN auto_routing_pool_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER model_name,
    ADD COLUMN authorized_amount BIGINT NOT NULL DEFAULT 0 AFTER video_route,
    ADD COLUMN settlement_transaction_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER spend_transaction_id,
    ADD INDEX idx_generation_jobs_auto_routing_pool_id (auto_routing_pool_id),
    ADD INDEX idx_generation_jobs_settlement_transaction_id (settlement_transaction_id);

UPDATE generation_jobs SET authorized_amount = billing_amount WHERE authorized_amount = 0;

-- +goose Down
ALTER TABLE generation_jobs
    DROP INDEX idx_generation_jobs_settlement_transaction_id,
    DROP INDEX idx_generation_jobs_auto_routing_pool_id,
    DROP COLUMN settlement_transaction_id,
    DROP COLUMN authorized_amount,
    DROP COLUMN auto_routing_pool_id;
DROP TABLE generation_attempts;
DROP TABLE auto_routing_pool_members;
DROP TABLE auto_routing_pools;
