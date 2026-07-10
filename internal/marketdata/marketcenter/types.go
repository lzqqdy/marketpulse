package marketcenter

// CenterResponse is the aggregated market center payload.
type CenterResponse struct {
	Market    string     `json:"market"`
	Source    string     `json:"source"`
	FetchedAt int64      `json:"fetchedAt"`
	ChgDiagram ChgDiagram `json:"chgdiagram"`
	Heatmap   Heatmap    `json:"heatmap"`
	Fundflow  Fundflow   `json:"fundflow"`
	Overview  Overview   `json:"overview"`
}

// ChgDiagram is the up/down distribution chart.
type ChgDiagram struct {
	TotalTitle string          `json:"totalTitle,omitempty"`
	TotalValue string          `json:"totalValue,omitempty"`
	Up         int             `json:"up"`
	Down       int             `json:"down"`
	Balance    int             `json:"balance"`
	Bars       []ChgDiagramBar `json:"bars"`
}

type ChgDiagramBar struct {
	Title  string `json:"title"`
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// Heatmap is a sector treemap source list.
type Heatmap struct {
	SortKey  string        `json:"sortKey"`
	TypeCode string        `json:"typeCode"`
	Items    []HeatmapItem `json:"items"`
}

type HeatmapItem struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Market        string  `json:"market"`
	Amount        string  `json:"amount,omitempty"`
	Volume        string  `json:"volume,omitempty"`
	MarketValue   string  `json:"marketValue,omitempty"`
	LastPx        string  `json:"lastPx,omitempty"`
	PxChangeRate  float64 `json:"pxChangeRate"`
	MetricValue   string  `json:"metricValue"`
	Logo          string  `json:"logo,omitempty"`
}

// Fundflow is main-force net inflow by block type.
type Fundflow struct {
	Groups []FundflowGroup `json:"groups"`
}

type FundflowGroup struct {
	BlockType     string           `json:"blockType"`
	BlockTypeName string           `json:"blockTypeName"`
	Items         []FundflowItem   `json:"items"`
}

type FundflowItem struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	MainNetTurnover string  `json:"mainNetTurnover"`
	NetAmount       float64 `json:"netAmount"`
}

// Overview is hot sector cards grouped by block type.
type Overview struct {
	Tabs []OverviewTab `json:"tabs"`
}

type OverviewTab struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Items    []OverviewItem `json:"items"`
}

type OverviewItem struct {
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	Price        float64          `json:"price"`
	ChangePct    float64          `json:"changePct"`
	ChangeStatus string           `json:"changeStatus"`
	LeadName     string           `json:"leadName,omitempty"`
	LeadChangePct float64         `json:"leadChangePct,omitempty"`
	Trend        []float64        `json:"trend,omitempty"`
}
