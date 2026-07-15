package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/alerts/channels"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/users"
)

// InboxPusher stores unread in-app items.
type InboxPusher interface {
	Push(ctx context.Context, userID int64, item InboxItem) error
}

// HubPusher pushes live WS alerts to online clients.
type HubPusher interface {
	PushAlert(userID int64, item InboxItem)
}

type dispatchJob struct {
	Rule         Rule
	TriggerValue float64
	Meta         triggerMeta
	Title        string
	Body         string
}

// Dispatcher handles async multi-channel delivery.
type Dispatcher struct {
	repo     *repository
	inbox    *InboxStore
	hub      *Hub
	email    *channels.Email
	pushplus *channels.PushPlus
	users    users.Service
	cooldown *CooldownStore
	onOnce   func(ctx context.Context, ruleID int64)
	jobs     chan dispatchJob
}

func NewDispatcher(
	repo *repository,
	inbox *InboxStore,
	hub *Hub,
	smtpCfg config.SMTPConfig,
	userSvc users.Service,
	cooldown *CooldownStore,
	onOnce func(ctx context.Context, ruleID int64),
) *Dispatcher {
	d := &Dispatcher{
		repo:     repo,
		inbox:    inbox,
		hub:      hub,
		email:    channels.NewEmail(smtpCfg),
		pushplus: channels.NewPushPlus(),
		users:    userSvc,
		cooldown: cooldown,
		onOnce:   onOnce,
		jobs:     make(chan dispatchJob, 128),
	}
	go d.worker()
	return d
}

// Enqueue schedules async delivery. Returns false when the worker queue is full
// so the caller can release a prematurely claimed cooldown.
func (d *Dispatcher) Enqueue(rule Rule, triggerValue float64, meta triggerMeta) bool {
	title, body := formatAlertMessage(rule, triggerValue, meta)
	select {
	case d.jobs <- dispatchJob{Rule: rule, TriggerValue: triggerValue, Meta: meta, Title: title, Body: body}:
		return true
	default:
		slog.Warn("alerts dispatcher queue full", "rule_id", rule.ID)
		return false
	}
}

func (d *Dispatcher) worker() {
	for job := range d.jobs {
		d.dispatch(context.Background(), job)
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, job dispatchJob) {
	rule := job.Rule
	now := time.Now().Unix()
	triggerStr := formatDecimal(job.TriggerValue)

	profile, profileErr := d.users.ProfileByID(ctx, rule.UserID)
	for _, ch := range rule.Channels {
		delivery := Delivery{
			RuleID:       rule.ID,
			UserID:       rule.UserID,
			AssetType:    rule.AssetType,
			Symbol:       rule.Symbol,
			RuleType:     rule.RuleType,
			Channel:      ch,
			TriggerValue: triggerStr,
			Title:        job.Title,
			Body:         job.Body,
			CreatedAt:    now,
		}
		var sendErr error
		switch ch {
		case ChannelInApp:
			rec, err := d.repo.InsertDelivery(ctx, deliveryWithStatus(delivery, DeliverySuccess, ""))
			if err != nil {
				slog.Error("alerts delivery insert", "err", err)
				continue
			}
			item := InboxItem{
				DeliveryID: rec.ID,
				RuleID:     rule.ID,
				Title:      job.Title,
				Body:       job.Body,
				Symbol:     rule.Symbol,
				CreatedAt:  now,
			}
			if err := d.inbox.Push(ctx, rule.UserID, item); err != nil {
				slog.Error("alerts inbox push", "err", err)
			}
			if d.hub != nil {
				d.hub.PushAlert(rule.UserID, item)
			}
		case ChannelEmail:
			if profileErr != nil {
				sendErr = profileErr
			} else if strings.TrimSpace(profile.Email) == "" {
				sendErr = fmt.Errorf("email not configured")
			} else if !d.email.Configured() {
				sendErr = fmt.Errorf("smtp not configured")
			} else {
				sendErr = d.email.Send(profile.Email, job.Title, job.Body)
			}
			status, errMsg := deliveryOutcome(sendErr)
			if sendErr != nil {
				slog.Warn("alerts email delivery", "rule_id", rule.ID, "status", status, "err", sendErr)
			}
			_, _ = d.repo.InsertDelivery(ctx, deliveryWithStatus(delivery, status, errMsg))
		case ChannelPushPlus:
			if profileErr != nil {
				sendErr = profileErr
			} else if strings.TrimSpace(profile.WechatPushToken) == "" {
				sendErr = fmt.Errorf("pushplus token not configured")
			} else {
				sendErr = d.pushplus.Send(profile.WechatPushToken, job.Title, job.Body)
			}
			status, errMsg := deliveryOutcome(sendErr)
			if sendErr != nil {
				slog.Warn("alerts pushplus delivery", "rule_id", rule.ID, "status", status, "err", sendErr)
			}
			_, _ = d.repo.InsertDelivery(ctx, deliveryWithStatus(delivery, status, errMsg))
		default:
			_, _ = d.repo.InsertDelivery(ctx, deliveryWithStatus(delivery, DeliverySkipped, "unknown channel"))
		}
	}

	_ = d.repo.RecordTrigger(ctx, rule.ID)
	// Cooldown was claimed in evaluator via TrySet before enqueue.
	if rule.Frequency == FrequencyOnce && d.onOnce != nil {
		d.onOnce(ctx, rule.ID)
	}
}

func deliveryWithStatus(d Delivery, status, errMsg string) Delivery {
	d.Status = status
	d.ErrorMsg = errMsg
	if len(d.ErrorMsg) > 512 {
		d.ErrorMsg = d.ErrorMsg[:512]
	}
	return d
}

func deliveryOutcome(err error) (status, errMsg string) {
	if err == nil {
		return DeliverySuccess, ""
	}
	if strings.Contains(err.Error(), "not configured") {
		return DeliverySkipped, err.Error()
	}
	return DeliveryFailed, err.Error()
}

// PushAlert sends a live WS alert event.
func (h *Hub) PushAlert(userID int64, item InboxItem) {
	payload, err := json.Marshal(map[string]any{
		"type": "alert",
		"data": map[string]any{
			"deliveryId": item.DeliveryID,
			"ruleId":     item.RuleID,
			"title":      item.Title,
			"body":       item.Body,
			"symbol":     item.Symbol,
			"createdAt":  item.CreatedAt,
		},
	})
	if err != nil {
		return
	}
	h.Push(userID, payload)
}
