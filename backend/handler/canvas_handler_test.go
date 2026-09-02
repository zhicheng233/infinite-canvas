package handler

import (
	"errors"
	"testing"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

func TestCanvasRepoConflictReturnsLatestPersistedProject(t *testing.T) {
	db := openAdminCreditTestDB(t)
	repo := repository.NewCanvasRepo(db)
	project := &model.CanvasProject{TenantID: 7, UserID: 9, ProjectID: "canvas-conflict", Title: "first"}

	created, err := repo.Save(project, 0)
	if err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	updated, err := repo.Save(&model.CanvasProject{TenantID: 7, UserID: 9, ProjectID: project.ProjectID, Title: "latest"}, created.Revision)
	if err != nil {
		t.Fatalf("update canvas: %v", err)
	}

	conflict, err := repo.Save(&model.CanvasProject{TenantID: 7, UserID: 9, ProjectID: project.ProjectID, Title: "stale client"}, created.Revision)
	if !errors.Is(err, repository.ErrCanvasRevisionConflict) {
		t.Fatalf("save error = %v, want revision conflict", err)
	}
	if conflict == nil || conflict.Title != updated.Title || conflict.Revision != updated.Revision {
		t.Fatalf("conflict project = %+v, want latest %+v", conflict, updated)
	}
}
