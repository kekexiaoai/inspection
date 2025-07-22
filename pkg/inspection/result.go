package inspection

import "time"

type Report struct {
	Template struct {
		Name        string    `json:"name"`
		DisplayName string    `json:"display_name"`
		ExecutedAt  time.Time `json:"executed_at"`
		ExecutedBy  string    `json:"executed_by"`
	} `json:"template"`
	SummaryOverviews []*SummaryOverview `json:"summary_overviews"`
	Sections         []*Section         `json:"sections"`
	Results          []*IndicatorResult `json:"results"`
}

type SummaryOverview struct {
	Indicator string `json:"indicator"`
	Unit      string `json:"unit"`
	Total     int    `json:"total"`
	Ok        int    `json:"ok"`
	Warning   int    `json:"warning"`
	Critical  int    `json:"critical"`
	Missing   int    `json:"missing"`
}

type IndicatorResult struct {
	Indicator     string            `json:"indicator"`
	Type          string            `json:"type"`
	Description   string            `json:"description"`
	Unit          string            `json:"unit"`
	DisplayType   string            `json:"display_type"`
	Summary       Summary           `json:"summary"`
	Page          PageInfo          `json:"page"`
	Highlight     HighlightInfo     `json:"highlight"`
	Values        []ValueItem       `json:"values"`
	Fields        []map[string]any  `json:"fields,omitempty"`
	StatusMapping map[string]string `json:"status_mapping,omitempty"`
}

type Summary struct {
	Total    int `json:"total"`
	Ok       int `json:"ok"`
	Info     int `json:"info"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Missing  int `json:"missing"`
}

type PageInfo struct {
	Size    int  `json:"size"`
	Index   int  `json:"index"`
	HasMore bool `json:"has_more"`
}

type HighlightInfo struct {
	Enabled bool        `json:"enabled"`
	Values  []ValueItem `json:"values"`
}

type ValueItem struct {
	Target  string   `json:"target"`
	Value   *float64 `json:"value"`
	Status  string   `json:"status,omitempty"`
	Missing bool     `json:"missing,omitempty"`
}
