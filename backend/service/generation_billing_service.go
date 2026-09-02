package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

type GenerationReservationInput struct {
	TenantID          uint
	UserID            uint
	Capability        string
	ModelName         string
	AutoRoutingPoolID uint
	ChannelID         uint
	ChannelModelID    uint
	ChannelName       string
	ChannelBaseURL    string
	VideoRoute        string
	Amount            int
	Note              string
	Metadata          string
}

type GenerationBillingService struct {
	repo *repository.GenerationJobRepo
}

func NewGenerationBillingService(repo *repository.GenerationJobRepo) *GenerationBillingService {
	return &GenerationBillingService{repo: repo}
}

func (s *GenerationBillingService) Reserve(input GenerationReservationInput) (*model.GenerationJob, int, error) {
	if s == nil || s.repo == nil {
		return nil, 0, errors.New("generation billing is not configured")
	}
	requestID, err := newGenerationRequestID()
	if err != nil {
		return nil, 0, err
	}
	job := &model.GenerationJob{
		RequestID: requestID, TenantID: input.TenantID, UserID: input.UserID,
		Capability: strings.TrimSpace(input.Capability), ModelName: strings.TrimSpace(input.ModelName), AutoRoutingPoolID: input.AutoRoutingPoolID,
		ChannelID: input.ChannelID, ChannelModelID: input.ChannelModelID,
		ChannelName: strings.TrimSpace(input.ChannelName), ChannelBaseURL: strings.TrimSpace(input.ChannelBaseURL),
		VideoRoute: strings.TrimSpace(input.VideoRoute), AuthorizedAmount: input.Amount, BillingAmount: input.Amount,
	}
	account, err := s.repo.Reserve(job, input.Note, input.Metadata)
	if err != nil {
		return nil, 0, err
	}
	return job, account.Balance, nil
}

type GenerationSettlementInput struct {
	Amount         int
	ChannelID      uint
	ChannelModelID uint
	ChannelName    string
	ChannelBaseURL string
	VideoRoute     string
	UpstreamTaskID string
}

func (s *GenerationBillingService) Settle(job *model.GenerationJob, input GenerationSettlementInput) (*repository.GenerationSettlementResult, error) {
	if job == nil {
		return nil, errors.New("generation job is required")
	}
	return s.repo.Settle(job.RequestID, repository.GenerationSettlementInput{
		Amount: input.Amount, ChannelID: input.ChannelID, ChannelModelID: input.ChannelModelID,
		ChannelName: strings.TrimSpace(input.ChannelName), ChannelBaseURL: strings.TrimSpace(input.ChannelBaseURL),
		VideoRoute: strings.TrimSpace(input.VideoRoute), UpstreamTaskID: strings.TrimSpace(input.UpstreamTaskID),
	})
}

func (s *GenerationBillingService) Succeed(job *model.GenerationJob, upstreamTaskID string) error {
	if job == nil {
		return nil
	}
	return s.repo.MarkSucceeded(job.RequestID, upstreamTaskID)
}

func (s *GenerationBillingService) Refund(job *model.GenerationJob, reason string) (*repository.GenerationRefundResult, error) {
	if job == nil {
		return &repository.GenerationRefundResult{}, nil
	}
	return s.repo.RefundByRequest(job.RequestID, reason)
}

func (s *GenerationBillingService) RefundTask(tenantID, userID, channelModelID uint, upstreamTaskID, reason string) (*repository.GenerationRefundResult, error) {
	return s.repo.RefundByTask(tenantID, userID, channelModelID, upstreamTaskID, reason)
}

func (s *GenerationBillingService) CompleteTask(tenantID, userID, channelModelID uint, upstreamTaskID string) error {
	return s.repo.CompleteByTask(tenantID, userID, channelModelID, upstreamTaskID)
}

func newGenerationRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
