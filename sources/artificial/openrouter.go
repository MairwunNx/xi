package artificial

import (
	"net/http"

	openrouter "github.com/revrost/go-openrouter"
)

func NewOpenRouterClient(config *AIConfig, client *http.Client) *openrouter.Client {
	clientConfig := openrouter.DefaultConfig(config.OpenRouterToken)
	clientConfig.HTTPClient = client
	clientConfig.XTitle = "Xi Manager"
	clientConfig.HttpReferer = "https://github.com/mairwunnx/xi"

	return openrouter.NewClientWithConfig(*clientConfig)
}