package alerts

import (
	"encoding/json"
)

const (
	AssetSpot  = "spot"
	AssetIndex = "index"

	FrequencyOnce  = "once"
	FrequencyLoop  = "loop"
	FrequencyDaily = "daily"

	StatusActive   = "active"
	StatusDisabled = "disabled"

	ChannelInApp    = "in_app"
	ChannelEmail    = "email"
	ChannelPushPlus = "pushplus"

	DeliverySuccess = "success"
	DeliveryFailed  = "failed"
	DeliverySkipped = "skipped"
)

var allowedChannels = map[string]struct{}{
	ChannelInApp:    {},
	ChannelEmail:    {},
	ChannelPushPlus: {},
}

// RuleParams holds threshold JSON for rule types 1–5.
type RuleParams struct {
	Target   *float64 `json:"target,omitempty"`
	Range    *float64 `json:"range,omitempty"`
	Upper    *float64 `json:"upper,omitempty"`
	Lower    *float64 `json:"lower,omitempty"`
	Ampl     *float64 `json:"ampl,omitempty"`
	RapidChg *float64 `json:"rapid_chg,omitempty"`
}

// Rule is a persisted alert rule.
type Rule struct {
	ID               int64           `json:"id"`
	UserID           int64           `json:"-"`
	AssetType        string          `json:"assetType"`
	Symbol           string          `json:"symbol"`
	Field            string          `json:"field"`
	RuleType         int             `json:"ruleType"`
	Params           RuleParams      `json:"params"`
	Channels         []string        `json:"channels"`
	Frequency        string          `json:"frequency"`
	IntervalMinutes  int             `json:"intervalMinutes"`
	SetPrice         string          `json:"setPrice"`
	Status           string          `json:"status"`
	LastTriggeredAt  *int64          `json:"lastTriggeredAt"`
	TriggerCount     int             `json:"triggerCount"`
	CreatedAt        int64           `json:"createdAt"`
	UpdatedAt        int64           `json:"updatedAt"`
}

// Delivery is a channel delivery record.
type Delivery struct {
	ID           int64  `json:"id"`
	RuleID       int64  `json:"ruleId"`
	UserID       int64  `json:"-"`
	AssetType    string `json:"assetType"`
	Symbol       string `json:"symbol"`
	RuleType     int     `json:"ruleType"`
	Channel      string `json:"channel"`
	TriggerValue string `json:"triggerValue"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	Status       string `json:"status"`
	ErrorMsg     string `json:"errorMsg"`
	CreatedAt    int64  `json:"createdAt"`
}

// InboxItem is an unread in-app alert stored in Redis.
type InboxItem struct {
	DeliveryID int64  `json:"deliveryId"`
	RuleID     int64  `json:"ruleId"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Symbol     string `json:"symbol"`
	CreatedAt  int64  `json:"createdAt"`
}

// CreateRuleInput is the API body for POST /rules.
type CreateRuleInput struct {
	AssetType       string          `json:"assetType"`
	Symbol          string          `json:"symbol"`
	Field           string          `json:"field"`
	RuleType        int             `json:"ruleType"`
	Params          json.RawMessage `json:"params"`
	Channels        []string        `json:"channels"`
	Frequency       string          `json:"frequency"`
	IntervalMinutes int             `json:"intervalMinutes"`
}

// UpdateRuleInput is the API body for PATCH /rules/:id.
type UpdateRuleInput struct {
	Params          *json.RawMessage `json:"params"`
	Channels        []string         `json:"channels"`
	Frequency       *string          `json:"frequency"`
	IntervalMinutes *int             `json:"intervalMinutes"`
	Status          *string          `json:"status"`
}

// ListRulesQuery holds pagination / filter / sort for rules.
type ListRulesQuery struct {
	Page      int
	PageSize  int
	Status    string
	AssetType string
	Symbol    string
	RuleType  int
	SortBy    string
	SortOrder string
}

// ListRulesResult is a paginated rule list.
type ListRulesResult struct {
	Items    []Rule `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Total    int    `json:"total"`
}

// ListDeliveriesQuery holds pagination filters.
type ListDeliveriesQuery struct {
	Page      int
	PageSize  int
	RuleID    int64
	Channel   string
	Status    string
	AssetType string
	Symbol    string
	RuleType  int
	SortBy    string
	SortOrder string
}

// ListDeliveriesResult is a paginated delivery list.
type ListDeliveriesResult struct {
	Items    []Delivery `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
	Total    int        `json:"total"`
}
