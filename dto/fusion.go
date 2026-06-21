package dto

type FusionAPIKeyCreateRequest struct {
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
}

type FusionAPIKeyUpdateRequest struct {
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	BaseURL      string   `json:"base_url"`
	APIKey       string   `json:"api_key,omitempty"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	Status       int      `json:"status"`
}

type FusionAPIKeyResponse struct {
	Id           int      `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	BaseURL      string   `json:"base_url"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	APIKeyHint   string   `json:"api_key_hint"`
	Status       int      `json:"status"`
	LastTestTime int64    `json:"last_test_time"`
	LastError    string   `json:"last_error"`
	CreatedAt    int64    `json:"created_at"`
	UpdatedAt    int64    `json:"updated_at"`
}

type FusionConfigCreateRequest struct {
	Name            string            `json:"name"`
	ModelAlias      string            `json:"model_alias"`
	Enabled         bool              `json:"enabled"`
	CandidateKeyIDs []int             `json:"candidate_key_ids"`
	CandidateModels map[string]string `json:"candidate_models"`
	JudgeKeyID      int               `json:"judge_key_id"`
	JudgeModel      string            `json:"judge_model"`
	Strategy        string            `json:"strategy"`
	TimeoutMS       int               `json:"timeout_ms"`
	MaxParallel     int               `json:"max_parallel"`
	MinSuccesses    int               `json:"min_successes"`
	JudgePrompt     string            `json:"judge_prompt"`
}

type FusionConfigUpdateRequest struct {
	FusionConfigCreateRequest
}

type FusionConfigResponse struct {
	Id              int               `json:"id"`
	Name            string            `json:"name"`
	ModelAlias      string            `json:"model_alias"`
	Enabled         bool              `json:"enabled"`
	CandidateKeyIDs []int             `json:"candidate_key_ids"`
	CandidateModels map[string]string `json:"candidate_models"`
	JudgeKeyID      int               `json:"judge_key_id"`
	JudgeModel      string            `json:"judge_model"`
	Strategy        string            `json:"strategy"`
	TimeoutMS       int               `json:"timeout_ms"`
	MaxParallel     int               `json:"max_parallel"`
	MinSuccesses    int               `json:"min_successes"`
	JudgePrompt     string            `json:"judge_prompt"`
	CreatedAt       int64             `json:"created_at"`
	UpdatedAt       int64             `json:"updated_at"`
}
