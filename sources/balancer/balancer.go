package balancer

import (
	"context"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	"github.com/mr-karan/balance"
)

type NeuroProvider interface {
	Response(ctx context.Context, log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair) (string, error)
}

type AIBalancer struct {
	balancer  *balance.Balance
	providers map[string]NeuroProvider
}

func NewAIBalancer(config *AIBalancerConfig, providers map[string]NeuroProvider) *AIBalancer {
	b := balance.NewBalance()

	for provider, weight := range config.Weights {
		b.Add(provider, weight)
	}

	return &AIBalancer{balancer: b, providers: providers}
}

func (x *AIBalancer) BalancedResponse(log *tracing.Logger, prompt string, req string, persona string, history []repository.MessagePair) (string, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	provider := x.GetNeuroProvider()
	return provider.Response(ctx, log, prompt, req, persona, history)
}

func (x *AIBalancer) GetNeuroProvider() NeuroProvider {
	return x.GetNeuroProviderByName(x.balancer.Get())
}

func (x *AIBalancer) GetNeuroProviderByName(name string) NeuroProvider {
	return x.providers[name]
}
