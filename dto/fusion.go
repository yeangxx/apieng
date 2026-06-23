package dto

type FusionAPIKeyCreateRequest struct {
	Name           string   `json:"name"`
	Provider       string   `json:"provider"`
	TemplateID     int      `json:"template_id"`
	BaseURL        string   `json:"base_url"`
	APIKey         string   `json:"api_key"`
	DefaultModel   string   `json:"default_model"`
	Models         []string `json:"models"`
	UpstreamConfig string   `json:"upstream_config"`
}

type FusionAPIKeyUpdateRequest struct {
	Name           string   `json:"name"`
	Provider       string   `json:"provider"`
	TemplateID     int      `json:"template_id"`
	BaseURL        string   `json:"base_url"`
	APIKey         string   `json:"api_key,omitempty"`
	DefaultModel   string   `json:"default_model"`
	Models         []string `json:"models"`
	UpstreamConfig string   `json:"upstream_config"`
	Status         int      `json:"status"`
}

type FusionAPIKeyResponse struct {
	Id             int      `json:"id"`
	Name           string   `json:"name"`
	Provider       string   `json:"provider"`
	TemplateID     int      `json:"template_id"`
	BaseURL        string   `json:"base_url"`
	DefaultModel   string   `json:"default_model"`
	Models         []string `json:"models"`
	UpstreamConfig string   `json:"upstream_config"`
	APIKeyHint     string   `json:"api_key_hint"`
	Status         int      `json:"status"`
	LastTestTime   int64    `json:"last_test_time"`
	LastError      string   `json:"last_error"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

type FusionUpstreamTemplateRequest struct {
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

type FusionUpstreamTemplateResponse struct {
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

type FusionAPIKeyTestRequest struct {
	FusionAPIKeyCreateRequest
}

type FusionAPIKeyTestResponse struct {
	OK             bool                   `json:"ok"`
	Status         int                    `json:"status"`
	Message        string                 `json:"message"`
	DetectedConfig map[string]interface{} `json:"detected_config"`
}

type FusionCandidateConfig struct {
	KeyID int    `json:"key_id"`
	Model string `json:"model"`
}

type FusionConfigCreateRequest struct {
	Name                  string                  `json:"name"`
	ModelAlias            string                  `json:"model_alias"`
	Enabled               bool                    `json:"enabled"`
	Candidates            []FusionCandidateConfig `json:"candidates"`
	CandidateKeyIDs       []int                   `json:"candidate_key_ids"`
	CandidateModels       map[string]string       `json:"candidate_models"`
	JudgeKeyID            int                     `json:"judge_key_id"`
	JudgeModel            string                  `json:"judge_model"`
	RoutingMode           string                  `json:"routing_mode"`
	DirectKeyID           int                     `json:"direct_key_id"`
	DirectModel           string                  `json:"direct_model"`
	QualityMode           string                  `json:"quality_mode"`
	RankerKeyID           int                     `json:"ranker_key_id"`
	RankerModel           string                  `json:"ranker_model"`
	EscalationKeyID       int                     `json:"escalation_key_id"`
	EscalationModel       string                  `json:"escalation_model"`
	QualityThreshold      float64                 `json:"quality_threshold"`
	RankerTopK            int                     `json:"ranker_top_k"`
	CandidateSamplingMode string                  `json:"candidate_sampling_mode"`
	Strategy              string                  `json:"strategy"`
	TimeoutMS             int                     `json:"timeout_ms"`
	MaxParallel           int                     `json:"max_parallel"`
	MinSuccesses          int                     `json:"min_successes"`
	JudgePrompt           string                  `json:"judge_prompt"`
}

type FusionConfigUpdateRequest struct {
	FusionConfigCreateRequest
}

type FusionConfigResponse struct {
	Id                    int                     `json:"id"`
	Name                  string                  `json:"name"`
	ModelAlias            string                  `json:"model_alias"`
	Enabled               bool                    `json:"enabled"`
	Candidates            []FusionCandidateConfig `json:"candidates"`
	CandidateKeyIDs       []int                   `json:"candidate_key_ids"`
	CandidateModels       map[string]string       `json:"candidate_models"`
	JudgeKeyID            int                     `json:"judge_key_id"`
	JudgeModel            string                  `json:"judge_model"`
	RoutingMode           string                  `json:"routing_mode"`
	DirectKeyID           int                     `json:"direct_key_id"`
	DirectModel           string                  `json:"direct_model"`
	QualityMode           string                  `json:"quality_mode"`
	RankerKeyID           int                     `json:"ranker_key_id"`
	RankerModel           string                  `json:"ranker_model"`
	EscalationKeyID       int                     `json:"escalation_key_id"`
	EscalationModel       string                  `json:"escalation_model"`
	QualityThreshold      float64                 `json:"quality_threshold"`
	RankerTopK            int                     `json:"ranker_top_k"`
	CandidateSamplingMode string                  `json:"candidate_sampling_mode"`
	Strategy              string                  `json:"strategy"`
	TimeoutMS             int                     `json:"timeout_ms"`
	MaxParallel           int                     `json:"max_parallel"`
	MinSuccesses          int                     `json:"min_successes"`
	JudgePrompt           string                  `json:"judge_prompt"`
	CreatedAt             int64                   `json:"created_at"`
	UpdatedAt             int64                   `json:"updated_at"`
}
