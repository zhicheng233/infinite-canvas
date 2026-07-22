package service

import (
	"strings"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type MergeGroupService struct {
	repo *repository.MergeGroupRepo
}

func NewMergeGroupService(repo *repository.MergeGroupRepo) *MergeGroupService {
	return &MergeGroupService{repo: repo}
}

func (s *MergeGroupService) ListByChannel(channelID uint) ([]model.ModelMergeGroup, error) {
	return s.repo.ListByChannel(channelID)
}

func (s *MergeGroupService) Create(group *model.ModelMergeGroup) error {
	return s.repo.Create(group)
}

func (s *MergeGroupService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *MergeGroupService) AutoCreate(channelID uint) ([]model.ModelMergeGroup, error) {
	names, err := s.repo.ListModelNames(channelID)
	if err != nil {
		return nil, err
	}

	// Group model names by their dashed prefixes (minimum 2 segments)
	prefixGroups := make(map[string][]string)
	for _, name := range names {
		parts := strings.Split(name, "-")
		for i := 2; i <= len(parts); i++ {
			prefix := strings.Join(parts[:i], "-")
			prefixGroups[prefix] = append(prefixGroups[prefix], name)
		}
	}

	// Delete existing merge groups for idempotency
	if err := s.repo.DeleteByChannel(channelID); err != nil {
		return nil, err
	}

	// Create merge groups for prefixes shared by >=2 models
	var created []model.ModelMergeGroup
	for prefix, models := range prefixGroups {
		if len(models) < 2 {
			continue
		}
		group := &model.ModelMergeGroup{
			ChannelID: channelID,
			GroupName: prefix,
			Pattern:   prefix,
			Enabled:   true,
		}
		if err := s.repo.Create(group); err != nil {
			return created, err
		}
		created = append(created, *group)
	}

	return created, nil
}
