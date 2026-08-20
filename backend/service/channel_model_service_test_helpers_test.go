package service

import (
	"errors"

	"infinite-canvas-server/model"
)

type channelModelRepoStub struct {
	item      *model.ChannelModel
	saveCalls int
}

func (repo *channelModelRepoStub) FindByID(uint) (*model.ChannelModel, error) {
	if repo.item == nil {
		return nil, errors.New("not found")
	}
	return repo.item, nil
}

func (*channelModelRepoStub) FindByChannelAndName(uint, string) (*model.ChannelModel, error) {
	return nil, errors.New("not implemented")
}

func (repo *channelModelRepoStub) ListByChannel(channelID uint, enabledOnly bool) ([]model.ChannelModel, error) {
	if repo.item == nil || repo.item.ChannelID != channelID || (enabledOnly && !repo.item.Enabled) {
		return []model.ChannelModel{}, nil
	}
	return []model.ChannelModel{*repo.item}, nil
}

func (repo *channelModelRepoStub) Save(*model.ChannelModel) error {
	repo.saveCalls++
	return nil
}

func (*channelModelRepoStub) Upsert(*model.ChannelModel) error {
	return errors.New("not implemented")
}

func (*channelModelRepoStub) DeleteStaleModels(uint, []string) error {
	return errors.New("not implemented")
}

type channelModelCatalogChannelServiceStub struct {
	channels []model.ChannelInfo
}

func (stub channelModelCatalogChannelServiceStub) ListEnabled() ([]model.ChannelInfo, error) {
	return stub.channels, nil
}

func (channelModelCatalogChannelServiceStub) DecryptedApiKey(uint) (string, error) {
	return "", errors.New("not implemented")
}

type channelModelCatalogChannelRepoStub struct {
	channel *model.Channel
}

func (stub channelModelCatalogChannelRepoStub) FindByID(uint) (*model.Channel, error) {
	return stub.channel, nil
}

func (channelModelCatalogChannelRepoStub) Save(*model.Channel) error { return nil }

type channelModelCatalogPricingStub struct {
	items map[string]map[uint]model.CreditPricing
}

func (stub channelModelCatalogPricingStub) FindPricingMap(uint) (map[string]map[uint]model.CreditPricing, error) {
	return stub.items, nil
}

func newChannelModelCatalogTestService(item *model.ChannelModel) *ChannelModelService {
	return NewChannelModelService(
		channelModelCatalogChannelServiceStub{channels: []model.ChannelInfo{{ID: item.ChannelID, Name: "A", Enabled: true}}},
		channelModelCatalogChannelRepoStub{channel: &model.Channel{BaseModel: model.BaseModel{ID: item.ChannelID}, Name: "A", Enabled: true}},
		&channelModelRepoStub{item: item},
		channelModelCatalogPricingStub{items: map[string]map[uint]model.CreditPricing{
			item.ModelName: {item.ChannelID: {ChannelID: item.ChannelID, Model: item.ModelName, CreditsPerUnit: 1, UnitType: model.UnitPerVideo}},
		}},
	)
}
