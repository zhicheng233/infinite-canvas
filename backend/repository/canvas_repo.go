package repository

import (
	"errors"
	"infinite-canvas-server/model"

	"gorm.io/gorm"
)

var ErrCanvasRevisionConflict = errors.New("canvas revision conflict")

type CanvasRepo struct {
	db *gorm.DB
}

func NewCanvasRepo(db *gorm.DB) *CanvasRepo {
	return &CanvasRepo{db: db}
}

func (r *CanvasRepo) Save(project *model.CanvasProject, expectedRevision uint) (*model.CanvasProject, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.CanvasProject
		err := tx.Where("tenant_id = ? AND user_id = ? AND project_id = ?", project.TenantID, project.UserID, project.ProjectID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if expectedRevision != 0 {
				return ErrCanvasRevisionConflict
			}
			project.Revision = 1
			if project.SchemaVersion <= 0 {
				project.SchemaVersion = 2
			}
			return tx.Create(project).Error
		}
		if err != nil {
			return err
		}
		if expectedRevision != existing.Revision {
			return ErrCanvasRevisionConflict
		}
		nextRevision := existing.Revision + 1
		result := tx.Model(&model.CanvasProject{}).
			Where("id = ? AND revision = ?", existing.ID, existing.Revision).
			Updates(map[string]any{
				"schema_version": project.SchemaVersion, "revision": nextRevision, "title": project.Title,
				"nodes": project.Nodes, "connections": project.Connections, "chat_sessions": project.ChatSessions,
				"active_chat_id": project.ActiveChatID, "background_mode": project.BackgroundMode,
				"show_image_info": project.ShowImageInfo, "viewport_x": project.ViewportX,
				"viewport_y": project.ViewportY, "viewport_k": project.ViewportK,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCanvasRevisionConflict
		}
		project.BaseModel = existing.BaseModel
		project.Revision = nextRevision
		return nil
	})
	if errors.Is(err, ErrCanvasRevisionConflict) {
		latest, latestErr := r.FindByProjectID(project.TenantID, project.UserID, project.ProjectID)
		if latestErr == nil {
			return latest, err
		}
		if !errors.Is(latestErr, gorm.ErrRecordNotFound) {
			return nil, latestErr
		}
		return nil, err
	}
	return project, err
}

func EnsureCanvasProjectIndexes(db *gorm.DB) error {
	migrator := db.Migrator()
	if migrator.HasIndex(&model.CanvasProject{}, "idx_canvas_projects_project_id") {
		if err := migrator.DropIndex(&model.CanvasProject{}, "idx_canvas_projects_project_id"); err != nil {
			return err
		}
	}
	if !migrator.HasIndex(&model.CanvasProject{}, "idx_canvas_project_identity") {
		return migrator.CreateIndex(&model.CanvasProject{}, "idx_canvas_project_identity")
	}
	return nil
}

func (r *CanvasRepo) FindByProjectID(tenantID uint, userID uint, projectID string) (*model.CanvasProject, error) {
	var p model.CanvasProject
	err := r.db.Where("tenant_id = ? AND user_id = ? AND project_id = ?", tenantID, userID, projectID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *CanvasRepo) ListByTenant(tenantID uint, userID uint) ([]model.CanvasProject, error) {
	var projects []model.CanvasProject
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Order("updated_at DESC").Find(&projects).Error
	return projects, err
}

func (r *CanvasRepo) Delete(tenantID uint, userID uint, projectID string) error {
	return r.db.Where("tenant_id = ? AND user_id = ? AND project_id = ?", tenantID, userID, projectID).Delete(&model.CanvasProject{}).Error
}

func (r *CanvasRepo) DeleteBatch(tenantID uint, userID uint, projectIDs []string) error {
	return r.db.Where("tenant_id = ? AND user_id = ? AND project_id IN ?", tenantID, userID, projectIDs).Delete(&model.CanvasProject{}).Error
}
