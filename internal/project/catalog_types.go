package project

type ProtocolOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Description string `json:"description"`
}

type CapabilityItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Path        string `json:"path"`
	Description string `json:"description"`
	DefaultOn   bool   `json:"default_on"`
	Required    bool   `json:"required"`
}
