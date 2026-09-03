package service

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
)

const (
	featureGuideMaxTitleLength = 100
	featureGuideMaxPages       = 20
	featureGuideMaxPageLength  = 20000
	featureGuideMaxTotalLength = 100000
)

var featureGuideDefaults = []model.FeatureGuidePayload{
	{Surface: model.FeatureGuideSurfaceCanvas, Title: "画布功能引导", Pages: []string{}, Version: 1},
	{Surface: model.FeatureGuideSurfaceImage, Title: "图片生成功能引导", Pages: []string{}, Version: 1},
	{Surface: model.FeatureGuideSurfaceVideo, Title: "视频生成功能引导", Pages: []string{}, Version: 1},
}

type FeatureGuideValidationError struct{ message string }

func (err *FeatureGuideValidationError) Error() string { return err.message }

func IsFeatureGuideValidationError(err error) bool {
	var validationError *FeatureGuideValidationError
	return errors.As(err, &validationError)
}

func newFeatureGuideValidationError(message string) error {
	return &FeatureGuideValidationError{message: message}
}

type featureGuideRepo interface {
	GetBySurface(surface model.FeatureGuideSurface) (*model.FeatureGuide, error)
	List() ([]model.FeatureGuide, error)
	UpdateLocked(surface model.FeatureGuideSurface, update func(*model.FeatureGuide) (*model.FeatureGuide, error)) (*model.FeatureGuide, error)
}

type FeatureGuideService struct{ repo featureGuideRepo }

func NewFeatureGuideService(repo featureGuideRepo) *FeatureGuideService {
	return &FeatureGuideService{repo: repo}
}

func (s *FeatureGuideService) GetPublic(surface model.FeatureGuideSurface) (*model.FeatureGuidePayload, error) {
	if !surface.Valid() {
		return nil, newFeatureGuideValidationError("不支持的功能引导类型")
	}
	item, err := s.repo.GetBySurface(surface)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	payload, err := decodeFeatureGuide(item)
	if err != nil {
		return nil, nil
	}
	if !payload.Enabled {
		return nil, nil
	}
	if _, err := normalizeFeatureGuide(*payload); err != nil {
		return nil, nil
	}
	return payload, nil
}

func (s *FeatureGuideService) ListAdmin() ([]model.FeatureGuidePayload, error) {
	items, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	bySurface := make(map[model.FeatureGuideSurface]model.FeatureGuidePayload, len(items))
	for index := range items {
		if !items[index].Surface.Valid() {
			continue
		}
		payload, err := decodeFeatureGuide(&items[index])
		if err != nil {
			payload = &model.FeatureGuidePayload{
				Surface: items[index].Surface,
				Enabled: false,
				Title:   items[index].Title,
				Pages:   []string{},
				Version: items[index].Version,
			}
		}
		bySurface[payload.Surface] = *payload
	}
	result := make([]model.FeatureGuidePayload, 0, len(featureGuideDefaults))
	for _, fallback := range featureGuideDefaults {
		if item, ok := bySurface[fallback.Surface]; ok {
			result = append(result, item)
		} else {
			fallback.Pages = append([]string{}, fallback.Pages...)
			result = append(result, fallback)
		}
	}
	return result, nil
}

func (s *FeatureGuideService) Save(surface model.FeatureGuideSurface, input model.FeatureGuidePayload) (*model.FeatureGuidePayload, error) {
	if !surface.Valid() {
		return nil, newFeatureGuideValidationError("不支持的功能引导类型")
	}
	input.Surface = surface
	normalized, err := normalizeFeatureGuide(input)
	if err != nil {
		return nil, err
	}
	pagesJSON, err := json.Marshal(normalized.Pages)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.UpdateLocked(surface, func(existing *model.FeatureGuide) (*model.FeatureGuide, error) {
		version := 1
		if existing != nil {
			current, err := decodeFeatureGuide(existing)
			version = existing.Version
			if version < 1 {
				version = 1
			}
			if err != nil || current.Title != normalized.Title || !slices.Equal(current.Pages, normalized.Pages) {
				version++
			}
		}
		return &model.FeatureGuide{Surface: surface, Enabled: normalized.Enabled, Title: normalized.Title, Pages: string(pagesJSON), Version: version}, nil
	})
	if err != nil {
		return nil, err
	}
	return decodeFeatureGuide(item)
}

func normalizeFeatureGuide(input model.FeatureGuidePayload) (model.FeatureGuidePayload, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Pages = append([]string(nil), input.Pages...)
	if input.Pages == nil {
		input.Pages = []string{}
	}
	if !input.Enabled {
		for len(input.Pages) > 0 && strings.TrimSpace(input.Pages[len(input.Pages)-1]) == "" {
			input.Pages = input.Pages[:len(input.Pages)-1]
		}
	}
	if utf8.RuneCountInString(input.Title) > featureGuideMaxTitleLength {
		return input, newFeatureGuideValidationError("引导标题最多 100 字")
	}
	if len(input.Pages) > featureGuideMaxPages {
		return input, newFeatureGuideValidationError("引导页数最多 20 页")
	}
	total := 0
	for _, page := range input.Pages {
		length := utf8.RuneCountInString(page)
		if length > featureGuideMaxPageLength {
			return input, newFeatureGuideValidationError("单页内容最多 20000 字")
		}
		total += length
		if input.Enabled && strings.TrimSpace(page) == "" {
			return input, newFeatureGuideValidationError("启用引导时每页内容不能为空")
		}
	}
	if total > featureGuideMaxTotalLength {
		return input, newFeatureGuideValidationError("引导总内容最多 100000 字")
	}
	if input.Enabled && len(input.Pages) == 0 {
		return input, newFeatureGuideValidationError("启用引导时至少需要一页内容")
	}
	return input, nil
}

func decodeFeatureGuide(item *model.FeatureGuide) (*model.FeatureGuidePayload, error) {
	pages := make([]string, 0)
	if err := json.Unmarshal([]byte(item.Pages), &pages); err != nil {
		return nil, err
	}
	if pages == nil {
		pages = []string{}
	}
	return &model.FeatureGuidePayload{
		Surface: item.Surface,
		Enabled: item.Enabled,
		Title:   item.Title,
		Pages:   pages,
		Version: item.Version,
	}, nil
}
