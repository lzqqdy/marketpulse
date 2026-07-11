package expressnews

// Response is the normalized express news API payload.
type Response struct {
	Tag       string     `json:"tag"`
	Pn        int        `json:"pn"`
	Rn        int        `json:"rn"`
	Source    string     `json:"source"`
	FetchedAt int64      `json:"fetchedAt"`
	HasMore   bool       `json:"hasMore"`
	Items     []NewsItem `json:"items"`
}

// NewsItem is a single express news row.
type NewsItem struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Body        string       `json:"body"`
	PublishTime int64        `json:"publishTime"`
	Provider    string       `json:"provider"`
	Tag         string       `json:"tag,omitempty"`
	Important   bool         `json:"important,omitempty"`
	ThirdURL    string       `json:"thirdUrl,omitempty"`
	Entities    []NewsEntity `json:"entities,omitempty"`
}

// NewsEntity is a related stock shown beside a news item.
type NewsEntity struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Market   string  `json:"market"`
	Exchange string  `json:"exchange,omitempty"`
	Price    string  `json:"price,omitempty"`
	Ratio    string  `json:"ratio,omitempty"`
	ChangePct float64 `json:"changePct,omitempty"`
	LogoURL  string  `json:"logoUrl,omitempty"`
}
