package repository

import (
	"infinite-canvas-server/model"

	"gorm.io/gorm"
)

type CanvasRepo struct {
	db *gorm.DB
}

func NewCanvasRepo(db *gorm.DB) *CanvasRepo {
	return &CanvasRepo{db: db}
}

func (r *CanvasRepo) Upsert(project *model.CanvasProject) error {
	return r.db.
		Where("tenant_id = ? AND user_id = ? AND project_id = ?", project.TenantID, project.UserID, project.ProjectID).
		Assign(project).
		FirstOrCreate(project).Error
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
