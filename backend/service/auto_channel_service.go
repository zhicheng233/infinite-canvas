package service

import (
	"sort"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type AutoChannelService struct {
	db               *gorm.DB
	channelRepo      *repository.ChannelRepo
	channelModelRepo *repository.ChannelModelRepo
}

func NewAutoChannelService(db *gorm.DB, channelRepo *repository.ChannelRepo, channelModelRepo *repository.ChannelModelRepo) *AutoChannelService {
	return &AutoChannelService{db: db, channelRepo: channelRepo, channelModelRepo: channelModelRepo}
}

type AggregatedChannelRef struct {
	ChannelID      uint    `json:"channel_id"`
	ChannelModelID uint    `json:"channel_model_id"`
	ChannelName    string  `json:"channel_name"`
	SuccessRate    float64 `json:"success_rate"`
}

type AggregatedModel struct {
	Model    string                 `json:"model"`
	Channels []AggregatedChannelRef `json:"channels"`
}

func (s *AutoChannelService) AggregateModels() ([]AggregatedModel, error) {
	channels, err := s.channelRepo.ListEnabled()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	grouped := make(map[string][]AggregatedChannelRef)

	for _, ch := range channels {
		models, err := s.channelModelRepo.ListByChannel(ch.ID, true)
		if err != nil {
			return nil, err
		}
		for _, cm := range models {
			rate := s.computeSuccessRate(cm.ID, cutoff)
			grouped[cm.ModelName] = append(grouped[cm.ModelName], AggregatedChannelRef{
				ChannelID:      ch.ID,
				ChannelModelID: cm.ID,
				ChannelName:    ch.Name,
				SuccessRate:    rate,
			})
		}
	}

	result := make([]AggregatedModel, 0, len(grouped))
	for name, refs := range grouped {
		sort.Slice(refs, func(i, j int) bool { return refs[i].ChannelID < refs[j].ChannelID })
		result = append(result, AggregatedModel{Model: name, Channels: refs})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Model < result[j].Model })

	return result, nil
}

func (s *AutoChannelService) computeSuccessRate(channelModelID uint, cutoff time.Time) float64 {
	var total, success int64
	s.db.Model(&model.ModelCallLog{}).Where("channel_model_id = ? AND created_at > ?", channelModelID, cutoff).Count(&total)
	if total == 0 {
		return 0
	}
	s.db.Model(&model.ModelCallLog{}).Where("channel_model_id = ? AND created_at > ? AND is_success = ?", channelModelID, cutoff, true).Count(&success)
	return successRatePercentage(total, success)
}

func successRatePercentage(total, success int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(success) / float64(total) * 100
}
