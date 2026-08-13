package service

import (
	"encoding/json"
	"errors"
	"strings"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

var ErrVideoConfigPresetNameConflict = errors.New("预设名称已存在")

type videoConfigPresetRepo interface {
	ListByTenant(tenantID uint) ([]model.VideoConfigPreset, error)
	Create(item *model.VideoConfigPreset) error
	DeleteByTenantAndID(tenantID, presetID uint) error
}

type VideoConfigPresetService struct {
	repo videoConfigPresetRepo
}

func NewVideoConfigPresetService(repo videoConfigPresetRepo) *VideoConfigPresetService {
	return &VideoConfigPresetService{repo: repo}
}

func (s *VideoConfigPresetService) List(tenantID uint) ([]model.VideoConfigPresetInfo, error) {
	items, err := s.repo.ListByTenant(tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]model.VideoConfigPresetInfo, 0, len(items))
	for i := range items {
		info, err := videoConfigPresetToInfo(&items[i])
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *VideoConfigPresetService) Create(tenantID uint, input model.CreateVideoConfigPresetInput) (*model.VideoConfigPresetInfo, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("预设名称不能为空")
	}
	if err := model.NormalizeAndValidateCustomVideoConfig(input.Config); err != nil {
		return nil, err
	}
	configJSON, err := json.Marshal(input.Config)
	if err != nil {
		return nil, err
	}
	item := &model.VideoConfigPreset{
		TenantID:       tenantID,
		Name:           name,
		NormalizedName: strings.ToLower(name),
		Config:         string(configJSON),
	}
	if err := s.repo.Create(item); err != nil {
		if errors.Is(err, repository.ErrVideoConfigPresetNameConflict) {
			return nil, ErrVideoConfigPresetNameConflict
		}
		return nil, err
	}
	info, err := videoConfigPresetToInfo(item)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *VideoConfigPresetService) Delete(tenantID, presetID uint) error {
	return s.repo.DeleteByTenantAndID(tenantID, presetID)
}

func videoConfigPresetToInfo(item *model.VideoConfigPreset) (model.VideoConfigPresetInfo, error) {
	var config model.CustomVideoConfig
	if err := json.Unmarshal([]byte(item.Config), &config); err != nil {
		return model.VideoConfigPresetInfo{}, err
	}
	if err := model.NormalizeAndValidateCustomVideoConfig(&config); err != nil {
		return model.VideoConfigPresetInfo{}, err
	}
	return model.VideoConfigPresetInfo{
		ID:        item.ID,
		Name:      item.Name,
		Config:    config,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}, nil
}
