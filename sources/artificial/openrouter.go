package artificial

import (
	"net/http"
	"ximanager/sources/configuration"

	openrouter "github.com/revrost/go-openrouter"
)

func NewOpenRouterClient(config *configuration.Config, client *http.Client) *openrouter.Client {
	clientConfig := openrouter.DefaultConfig(config.AI.OpenRouterToken)
	clientConfig.HTTPClient = client
	clientConfig.XTitle = "Emperor Xi"
	clientConfig.HttpReferer = "https://github.com/mairwunnx/xi"

	return openrouter.NewClientWithConfig(*clientConfig)
}