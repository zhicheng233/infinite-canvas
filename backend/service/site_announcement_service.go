package service

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

type siteAnnouncementRepo interface {
	Get() (*model.SiteAnnouncement, error)
	Save(item *model.SiteAnnouncement) error
}

type SiteAnnouncementService struct {
	repo siteAnnouncementRepo
}

func NewSiteAnnouncementService(repo siteAnnouncementRepo) *SiteAnnouncementService {
	return &SiteAnnouncementService{repo: repo}
}

func (s *SiteAnnouncementService) GetPublic() (*model.SiteAnnouncementPayload, error) {
	item, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.SiteAnnouncementPayload{Enabled: false}, nil
		}
		return nil, err
	}
	if !item.Enabled || strings.TrimSpace(item.Content) == "" {
		return &model.SiteAnnouncementPayload{Enabled: false, Version: item.Version}, nil
	}
	return toSiteAnnouncementPayload(item), nil
}

func (s *SiteAnnouncementService) GetAdmin() (*model.SiteAnnouncementPayload, error) {
	item, err := s.repo.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.SiteAnnouncementPayload{Enabled: false, Title: "公告", Version: 1}, nil
		}
		return nil, err
	}
	return toSiteAnnouncementPayload(item), nil
}

func (s *SiteAnnouncementService) Save(input model.SiteAnnouncementPayload) (*model.SiteAnnouncementPayload, error) {
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.Content)
	if input.Enabled && content == "" {
		return nil, errors.New("公告内容不能为空")
	}
	if title == "" {
		title = "公告"
	}

	version := 1
	existing, err := s.repo.Get()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		version = existing.Version
		if existing.Enabled != input.Enabled || strings.TrimSpace(existing.Title) != title || strings.TrimSpace(existing.Content) != content {
			version += 1
		}
	}

	item := &model.SiteAnnouncement{Enabled: input.Enabled, Title: title, Content: content, Version: version}
	if err := s.repo.Save(item); err != nil {
		return nil, err
	}
	return &model.SiteAnnouncementPayload{Enabled: input.Enabled, Title: title, Content: content, Version: version}, nil
}

func toSiteAnnouncementPayload(item *model.SiteAnnouncement) *model.SiteAnnouncementPayload {
	return &model.SiteAnnouncementPayload{
		Enabled: item.Enabled,
		Title:   item.Title,
		Content: item.Content,
		Version: item.Version,
	}
}
