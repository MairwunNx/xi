package artificial

type ModelMeta struct {
	Name            string `json:"name"`
	AAI             int    `json:"aai"`
	InputPricePerM  string `json:"input_price_per_m"`
	OutputPricePerM string `json:"output_price_per_m"`
	CtxTokens       string `json:"ctx_tokens"`
}

type ContextLimits struct {
	TTL         int
	MaxMessages int
	MaxTokens   int
}

type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

type StreamCallback func(chunk StreamChunk)