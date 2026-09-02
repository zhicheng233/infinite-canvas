package service

import (
	"errors"
	"net/url"
	"strings"

	"infinite-canvas-server/crypto"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

var ErrChannelConfigRevisionConflict = errors.New("渠道配置已被其他管理员修改，请刷新后重试")

type ChannelService struct {
	repo       *repository.ChannelRepo
	encryptKey string
}

func NewChannelService(repo *repository.ChannelRepo, encryptKey string) *ChannelService {
	return &ChannelService{repo: repo, encryptKey: encryptKey}
}

func (s *ChannelService) Create(input model.SaveChannelInput) (*model.ChannelAdminInfo, error) {
	name, baseURL, err := validateChannelInput(input.Name, input.BaseUrl)
	if err != nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(input.ApiKey)
	if apiKey == "" {
		return nil, errors.New("api key is required")
	}

	encryptedKey, err := crypto.Encrypt(s.encryptKey, apiKey)
	if err != nil {
		return nil, err
	}

	if len([]rune(input.Remark)) > 500 {
		return nil, errors.New("备注不能超过500个字符")
	}
	videoAPIStandard, err := validateVideoAPIStandard(input.VideoAPIStandard)
	if err != nil {
		return nil, err
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	channel := &model.Channel{
		Name:             name,
		BaseUrl:          baseURL,
		ApiKey:           encryptedKey,
		Enabled:          enabled,
		VideoAPIStandard: videoAPIStandard,
		NewApiChannelID:  input.NewApiChannelID,
		MetricsBaseUrl:   input.MetricsBaseUrl,
		Remark:           input.Remark,
		ConfigRevision:   1,
	}
	if err := s.repo.Create(channel); err != nil {
		return nil, err
	}

	info := channelToAdminInfo(channel)
	return &info, nil
}

func (s *ChannelService) Update(id uint, input model.SaveChannelInput) (*model.ChannelAdminInfo, error) {
	name, baseURL, err := validateChannelInput(input.Name, input.BaseUrl)
	if err != nil {
		return nil, err
	}

	channel, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	currentRevision := channel.ConfigRevision
	if currentRevision == 0 {
		currentRevision = 1
	}
	if input.ExpectedRevision != nil && *input.ExpectedRevision != currentRevision {
		return nil, ErrChannelConfigRevisionConflict
	}

	if len([]rune(input.Remark)) > 500 {
		return nil, errors.New("备注不能超过500个字符")
	}
	if input.VideoAPIStandard != nil {
		videoAPIStandard, err := validateVideoAPIStandard(input.VideoAPIStandard)
		if err != nil {
			return nil, err
		}
		channel.VideoAPIStandard = videoAPIStandard
	}

	channel.Name = name
	channel.BaseUrl = baseURL
	if input.Enabled != nil {
		channel.Enabled = *input.Enabled
	}
	channel.NewApiChannelID = input.NewApiChannelID
	channel.MetricsBaseUrl = input.MetricsBaseUrl
	channel.Remark = input.Remark

	apiKey := strings.TrimSpace(input.ApiKey)
	if apiKey != "" {
		encryptedKey, err := crypto.Encrypt(s.encryptKey, apiKey)
		if err != nil {
			return nil, err
		}
		channel.ApiKey = encryptedKey
	}
	channel.ConfigRevision = currentRevision + 1

	saved, err := s.repo.SaveWithRevision(channel, currentRevision)
	if err != nil {
		return nil, err
	}
	if !saved {
		return nil, ErrChannelConfigRevisionConflict
	}

	info := channelToAdminInfo(channel)
	return &info, nil
}

func (s *ChannelService) Disable(id uint) error {
	return s.repo.Disable(id)
}

func (s *ChannelService) Enable(id uint) error {
	return s.repo.Enable(id)
}

func (s *ChannelService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *ChannelService) ListAll() ([]model.ChannelAdminInfo, error) {
	channels, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}
	return channelsToAdminInfo(channels), nil
}

func (s *ChannelService) ListEnabled() ([]model.ChannelInfo, error) {
	channels, err := s.repo.ListEnabled()
	if err != nil {
		return nil, err
	}
	return channelsToInfo(channels), nil
}

func (s *ChannelService) Get(id uint) (*model.ChannelAdminInfo, error) {
	channel, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	info := channelToAdminInfo(channel)
	return &info, nil
}

func (s *ChannelService) DecryptedApiKey(id uint) (string, error) {
	channel, err := s.repo.FindByID(id)
	if err != nil {
		return "", err
	}
	return crypto.Decrypt(s.encryptKey, channel.ApiKey)
}

func validateChannelInput(nameInput, baseURLInput string) (string, string, error) {
	name := strings.TrimSpace(nameInput)
	if name == "" {
		return "", "", errors.New("name is required")
	}
	if len([]rune(name)) > 100 {
		return "", "", errors.New("name must be at most 100 characters")
	}

	baseURL := strings.TrimSpace(baseURLInput)
	parsedURL, err := url.ParseRequestURI(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", "", errors.New("base_url must be a valid http or https URL")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", "", errors.New("base_url must be a valid http or https URL")
	}

	return name, baseURL, nil
}

func validateVideoAPIStandard(input *string) (string, error) {
	if input == nil || strings.TrimSpace(*input) == "" {
		return model.VideoAPIStandardDefault, nil
	}
	value := strings.ToLower(strings.TrimSpace(*input))
	if value != model.VideoAPIStandardDefault && value != model.VideoAPIStandardBinghuo {
		return "", errors.New("video_api_standard must be default or binghuo")
	}
	return value, nil
}

func channelsToInfo(channels []model.Channel) []model.ChannelInfo {
	infos := make([]model.ChannelInfo, 0, len(channels))
	for index := range channels {
		infos = append(infos, channelToInfo(&channels[index]))
	}
	return infos
}

func channelsToAdminInfo(channels []model.Channel) []model.ChannelAdminInfo {
	infos := make([]model.ChannelAdminInfo, 0, len(channels))
	for index := range channels {
		infos = append(infos, channelToAdminInfo(&channels[index]))
	}
	return infos
}

func channelToInfo(channel *model.Channel) model.ChannelInfo {
	return model.ChannelInfo{
		ID:               channel.ID,
		Name:             channel.Name,
		Enabled:          channel.Enabled,
		VideoAPIStandard: normalizeChannelVideoAPIStandard(channel.VideoAPIStandard),
		NewApiChannelID:  channel.NewApiChannelID,
		MetricsBaseUrl:   channel.MetricsBaseUrl,
		SyncStatus:       channel.SyncStatus,
		SyncError:        channel.SyncError,
		SyncedAt:         channel.SyncedAt,
	}
}

func normalizeChannelVideoAPIStandard(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), model.VideoAPIStandardBinghuo) {
		return model.VideoAPIStandardBinghuo
	}
	return model.VideoAPIStandardDefault
}

func channelToAdminInfo(channel *model.Channel) model.ChannelAdminInfo {
	return model.ChannelAdminInfo{
		ChannelInfo: channelToInfo(channel),
		BaseUrl:     channel.BaseUrl,
		HasKey:      channel.ApiKey != "",
		Remark:      channel.Remark,
	}
}
