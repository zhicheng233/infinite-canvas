-- +goose Up
CREATE TABLE IF NOT EXISTS feature_guides (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    created_at DATETIME(3) NULL,
    updated_at DATETIME(3) NULL,
    deleted_at DATETIME(3) NULL,
    surface VARCHAR(20) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    title VARCHAR(100) NOT NULL DEFAULT '',
    pages LONGTEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    UNIQUE INDEX idx_feature_guides_surface (surface),
    INDEX idx_feature_guides_deleted_at (deleted_at),
    CONSTRAINT chk_feature_guides_surface CHECK (surface IN ('canvas', 'image', 'video'))
);

INSERT IGNORE INTO feature_guides (surface, enabled, title, pages, version) VALUES
    ('canvas', FALSE, '画布功能引导', '[]', 1),
    ('image', FALSE, '图片生成功能引导', '[]', 1),
    ('video', FALSE, '视频生成功能引导', '[]', 1);

-- +goose Down
DROP TABLE IF EXISTS feature_guides;
