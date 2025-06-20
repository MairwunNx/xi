package balancer

import "ximanager/sources/platform"

type AIBalancerConfig struct {
	Weights map[string]int
}

func NewAIBalancerConfig() *AIBalancerConfig {
	return &AIBalancerConfig{Weights: map[string]int{
		"openai":   platform.GetAsInt("AI_OPENAI_WEIGHT", 40),
		"grok":     platform.GetAsInt("AI_GROK_WEIGHT", 25),
		"claude":   platform.GetAsInt("AI_CLAUDE_WEIGHT", 20),
		"deepseek": platform.GetAsInt("AI_DEEPSEEK_WEIGHT", 15),
	}}
}