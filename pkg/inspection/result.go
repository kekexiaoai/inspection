package inspection

type IndicatorResult struct {
	Indicator   string           `json:"indicator"`
	Type        string           `json:"type"`
	Unit        string           `json:"unit"`
	DisplayType string           `json:"display_type"`
	Summary     Summary          `json:"summary"`
	Page        PageInfo         `json:"page"`
	Highlight   HighlightInfo    `json:"highlight"`
	Values      []ValueItem      `json:"values"`
	Fields      []map[string]any `json:"fields,omitempty"`
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
	TopN   int         `json:"top_n"`
	Values []ValueItem `json:"values"`
}

type ValueItem struct {
	Target  string   `json:"target"`
	Value   *float64 `json:"value"`
	Status  string   `json:"status,omitempty"`
	Missing bool     `json:"missing,omitempty"`
}
