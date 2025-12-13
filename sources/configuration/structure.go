package configuration

import (
	"time"
)

type Config struct {
	Service      ServiceConfig      `yaml:"service"`
	Database     DatabaseConfig     `yaml:"database"`
	Redis        RedisConfig        `yaml:"redis"`
	Telegram     TelegramConfig     `yaml:"telegram"`
	AI           AIConfig           `yaml:"ai"`
	Proxy        ProxyConfig        `yaml:"proxy"`
	Network      NetworkConfig      `yaml:"network"`
	Throttler    ThrottlerConfig    `yaml:"throttler"`
	Features     FeaturesConfig     `yaml:"features"`
	Localization LocalizationConfig `yaml:"localization"`
}

type ServiceConfig struct {
	StartupPort            int `yaml:"startup_port"`
	SystemMetricsPort      int `yaml:"system_metrics_port"`
	ApplicationMetricsPort int `yaml:"application_metrics_port"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"ssl_mode"`
	TimeZone string `yaml:"time_zone"`
}

type RedisConfig struct {
	Host        string        `yaml:"host"`
	Port        int           `yaml:"port"`
	Password    string        `yaml:"password"`
	DB          int           `yaml:"db"`
	MaxRetries  int           `yaml:"max_retries"`
	DialTimeout time.Duration `yaml:"dial_timeout"`
	MessagesTTL time.Duration `yaml:"messages_ttl"`
}

type TelegramConfig struct {
	BotToken          string   `yaml:"bot_token"`
	APIEndpoint       string   `yaml:"api_endpoint"`
	FileAPIEndpoint   string   `yaml:"file_api_endpoint"`
	PollerTimeout     int      `yaml:"poller_timeout"`
	AllowedUpdates    []string `yaml:"allowed_updates"`
	DiplomatChunkSize int      `yaml:"diplomat_chunk_size"`
}

type AIConfig struct {
	OpenRouterToken string `yaml:"open_router_token"`
	OpenAIToken     string `yaml:"openai_token"`

	WhisperModel string `yaml:"whisper_model"`

	Agents  AI_AgentsConfig  `yaml:"agents"`
	Prompts AI_PromptsConfig `yaml:"prompts"`

	PlaceboModels []string `yaml:"placebo_models"`

	LimitExceededModel          string   `yaml:"limit_exceeded_model"`
	LimitExceededFallbackModels []string `yaml:"limit_exceeded_fallback_models"`

	TariffModels AI_TariffModelsConfig `yaml:"tariff_models"`
}

type AI_AgentsConfig struct {
	EffortSelection          AI_AgentConfig         `yaml:"effort_selection"`
	ResponseLength           AI_AgentConfig         `yaml:"response_length"`
	Summarization            AI_SummarizationConfig `yaml:"summarization"`
	WebSearch                AI_WebSearchConfig     `yaml:"web_search"`
	PersonalizationExtractor AI_AgentConfig         `yaml:"personalization_extractor"`
}

type AI_AgentConfig struct {
	Model   string `yaml:"model"`
	Timeout int    `yaml:"timeout"`
}

type AI_SummarizationConfig struct {
	Model                   string `yaml:"model"`
	Timeout                 int    `yaml:"timeout"`
	RecentMessagesToKeep    int    `yaml:"recent_messages_to_keep"`
	TriggerThresholdPercent int    `yaml:"trigger_threshold_percent"`
	MaxContextTokens        int    `yaml:"max_context_tokens"`
}

type AI_WebSearchConfig struct {
	DeepModel        string `yaml:"deep_model"`
	BriefModel       string `yaml:"brief_model"`
	FallbackModel    string `yaml:"fallback_model"`
	Timeout          int    `yaml:"timeout"`
	ReasoningEffort  string `yaml:"reasoning_effort"`
	MaxCallsPerQuery int    `yaml:"max_calls_per_query"`
}

type AI_PromptsConfig struct {
	EffortSelection           string `yaml:"effort_selection"`
	ResponseLength            string `yaml:"response_length"`
	Summarization             string `yaml:"summarization"`
	PersonalizationValidation string `yaml:"personalization_validation"`
	PersonalizationExtraction string `yaml:"personalization_extraction"`
	WebSearch                 string `yaml:"web_search"`
}

type AI_TariffModelsConfig struct {
	Bronze AI_TariffModelConfig `yaml:"bronze"`
	Silver AI_TariffModelConfig `yaml:"silver"`
	Gold   AI_TariffModelConfig `yaml:"gold"`
}

type AI_TariffModelConfig struct {
	PrimaryModel  string   `yaml:"primary_model"`
	FallbackModel string   `yaml:"fallback_model"`
}

type ProxyConfig struct {
	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type NetworkConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

type ThrottlerConfig struct {
	Limit time.Duration `yaml:"limit"`
}

type FeaturesConfig struct {
	UnleashAPIURL     string `yaml:"unleash_api_url"`
	UnleashAppName    string `yaml:"unleash_app_name"`
	UnleashInstanceID string `yaml:"unleash_instance_id"`
	RefreshInterval   int    `yaml:"refresh_interval"`
}

type LocalizationConfig struct {
	SupportedLanguages []string `yaml:"supported_languages"`
}
