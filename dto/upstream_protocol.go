package dto

type UpstreamProtocolTemplateRequest struct {
	Name                 string `json:"name"`
	ProviderLabel        string `json:"provider_label"`
	Protocol             string `json:"protocol"`
	ClientPath           string `json:"client_path"`
	EndpointPath         string `json:"endpoint_path"`
	AuthType             string `json:"auth_type"`
	AuthHeader           string `json:"auth_header"`
	AuthQueryName        string `json:"auth_query_name"`
	DefaultHeaders       string `json:"default_headers"`
	DefaultQuery         string `json:"default_query"`
	DefaultBodyOverrides string `json:"default_body_overrides"`
	RequestConverter     string `json:"request_converter"`
	ResponseConverter    string `json:"response_converter"`
	StreamConverter      string `json:"stream_converter"`
	DetectRules          string `json:"detect_rules"`
	Enabled              bool   `json:"enabled"`
	Sort                 int    `json:"sort"`
}

type UpstreamProtocolTemplateResponse struct {
	Id                   int    `json:"id"`
	Name                 string `json:"name"`
	ProviderLabel        string `json:"provider_label"`
	Protocol             string `json:"protocol"`
	ClientPath           string `json:"client_path"`
	EndpointPath         string `json:"endpoint_path"`
	AuthType             string `json:"auth_type"`
	AuthHeader           string `json:"auth_header"`
	AuthQueryName        string `json:"auth_query_name"`
	DefaultHeaders       string `json:"default_headers"`
	DefaultQuery         string `json:"default_query"`
	DefaultBodyOverrides string `json:"default_body_overrides"`
	RequestConverter     string `json:"request_converter"`
	ResponseConverter    string `json:"response_converter"`
	StreamConverter      string `json:"stream_converter"`
	DetectRules          string `json:"detect_rules"`
	Enabled              bool   `json:"enabled"`
	Sort                 int    `json:"sort"`
	CreatedAt            int64  `json:"created_at"`
	UpdatedAt            int64  `json:"updated_at"`
}
