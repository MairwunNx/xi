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

type StreamStatus string

const (
	StreamStatusNone      StreamStatus = ""
	StreamStatusThinking  StreamStatus = "thinking"
	StreamStatusSearching StreamStatus = "searching"
)

type StreamChunk struct {
	Content string
	Done    bool
	Error   error
	Status  StreamStatus // Optional status indicator (thinking, searching, etc.)
}

type StreamCallback func(chunk StreamChunk)