package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/repository"
)

// WebhookPoller periodically checks model availability via channel_models and
// sends webhook notifications when model state (up/down) changes.
type WebhookPoller struct {
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	running          bool
	wg               sync.WaitGroup
	interval         time.Duration
	webhookRepo      *repository.WebhookRepo
	channelRepo      *repository.ChannelRepo
	channelModelRepo *repository.ChannelModelRepo
	db               *gorm.DB
	sender           WebhookSender
	states           map[string]string // model_name -> "up" | "down"
}

// NewWebhookPoller creates a poller. db is required for cross-tenant queries
// on channel_models / webhook_configs that the existing repos do not expose.
func NewWebhookPoller(
	webhookRepo *repository.WebhookRepo,
	channelRepo *repository.ChannelRepo,
	channelModelRepo *repository.ChannelModelRepo,
	db *gorm.DB,
	sender WebhookSender,
) *WebhookPoller {
	return &WebhookPoller{
		webhookRepo:      webhookRepo,
		channelRepo:      channelRepo,
		channelModelRepo: channelModelRepo,
		db:               db,
		sender:           sender,
		interval:         5 * time.Minute,
		states:           make(map[string]string),
	}
}

// Start begins the poller goroutine. Safe to call multiple times.
func (p *WebhookPoller) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.running = true
	p.wg.Add(1)
	go p.loop()
	return nil
}

// Stop cancels the poller context and waits for the goroutine to exit.
func (p *WebhookPoller) Stop() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// IntervalSeconds returns the poll interval in seconds.
func (p *WebhookPoller) IntervalSeconds() int {
	return int(p.interval.Seconds())
}

// IsRunning reports whether the poller goroutine is active.
func (p *WebhookPoller) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *WebhookPoller) loop() {
	defer p.wg.Done()
	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkOnce()
		case <-p.ctx.Done():
			return
		}
	}
}

// checkOnce queries channel_models to determine model availability, compares
// against tracked state, and dispatches webhook notifications on state changes.
func (p *WebhookPoller) checkOnce() {
	// 1. Query all unique model names from channel_models
	var allModels []string
	if err := p.db.Model(&model.ChannelModel{}).
		Distinct("model_name").
		Pluck("model_name", &allModels).Error; err != nil {
		log.Printf("webhook poller: query models: %v", err)
		return
	}

	// 2. Build set of models that are: channel.enabled=true AND channel_model.enabled=true
	var enabledChannels []model.Channel
	if err := p.db.Where("enabled = ?", true).Find(&enabledChannels).Error; err != nil {
		log.Printf("webhook poller: query channels: %v", err)
		return
	}

	availableModels := make(map[string]bool)
	if len(enabledChannels) > 0 {
		channelIDs := make([]uint, len(enabledChannels))
		for i, ch := range enabledChannels {
			channelIDs[i] = ch.ID
		}
		var available []string
		if err := p.db.Model(&model.ChannelModel{}).
			Where("channel_id IN ? AND enabled = ?", channelIDs, true).
			Distinct("model_name").
			Pluck("model_name", &available).Error; err != nil {
			log.Printf("webhook poller: query channel models: %v", err)
			return
		}
		for _, m := range available {
			availableModels[m] = true
		}
	}

	now := time.Now()
	for _, modelName := range allModels {
		newState := "down"
		if availableModels[modelName] {
			newState = "up"
		}

		p.mu.Lock()
		oldState, exists := p.states[modelName]
		p.mu.Unlock()

		if !exists || oldState != newState {
			p.notifyStateChange(modelName, newState, now)

			p.mu.Lock()
			p.states[modelName] = newState
			p.mu.Unlock()
		}
	}
}

// notifyStateChange sends webhook notifications to all tenants with enabled
// webhook configs, respecting per-config cooldown.
func (p *WebhookPoller) notifyStateChange(modelName, newState string, now time.Time) {
	var configs []model.WebhookConfig
	if err := p.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		log.Printf("webhook poller: query webhook configs: %v", err)
		return
	}

	for _, cfg := range configs {
		// Cooldown check: skip if a log entry for this (tenant, model, state)
		// was created within the cooldown window.
		lastLog, err := p.webhookRepo.LastLogForModel(cfg.TenantID, modelName, newState)
		if err == nil {
			cooldownDeadline := lastLog.CreatedAt.Add(time.Duration(cfg.CooldownMinutes) * time.Minute)
			if now.Before(cooldownDeadline) {
				skipLog := &model.WebhookLog{
					TenantID:        cfg.TenantID,
					Platform:        cfg.Platform,
					ModelName:       modelName,
					Status:          newState,
					CooldownSkipped: true,
				}
				if err := p.webhookRepo.InsertLog(skipLog); err != nil {
					log.Printf("webhook poller: insert cooldown-skip log: %v", err)
				}
				continue
			}
		}
		// err == gorm.ErrRecordNotFound is expected — no prior log means no cooldown.

		// Render template
		template := cfg.TemplateDown
		if newState == "up" {
			template = cfg.TemplateUp
		}
		message := renderTemplate(template, modelName, newState, now)

		// Send via platform-specific sender
		s := NewSender(cfg.Platform)
		if s == nil {
			log.Printf("webhook poller: unknown platform %q for tenant %d", cfg.Platform, cfg.TenantID)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		sendErr := s.Send(ctx, cfg.WebhookURL, message)
		cancel()

		logEntry := &model.WebhookLog{
			TenantID:        cfg.TenantID,
			Platform:        cfg.Platform,
			ModelName:       modelName,
			Status:          newState,
			Message:         message,
			Success:         sendErr == nil,
			CooldownSkipped: false,
		}
		if sendErr != nil {
			logEntry.ResponseBody = sendErr.Error()
			log.Printf("webhook poller: send %s to tenant %d: %v", cfg.Platform, cfg.TenantID, sendErr)
		}
		if err := p.webhookRepo.InsertLog(logEntry); err != nil {
			log.Printf("webhook poller: insert webhook log: %v", err)
		}
	}
}

// renderTemplate replaces {{model}}, {{status}}, and {{time}} placeholders.
func renderTemplate(tmpl, modelName, status string, t time.Time) string {
	replacer := strings.NewReplacer(
		"{{model}}", modelName,
		"{{status}}", status,
		"{{time}}", t.Format(time.RFC3339),
	)
	return replacer.Replace(tmpl)
}
