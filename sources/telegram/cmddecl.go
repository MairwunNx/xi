package telegram

type ChatID int64

type ModeCmd struct {
	Add struct {
		ChatID ChatID `arg:"" name:"chat_id" help:"Chat ID (use ~ for negative IDs)"`
		Type   string `arg:"" name:"type" help:"Mode type"`
		Name   string `arg:"" name:"name" help:"Mode name"`
		Prompt string `arg:"" name:"prompt" help:"Mode prompt"`
	} `cmd:"" help:"Add a new mode"`

	List struct {
		ChatID ChatID `arg:"" name:"chat_id" help:"Chat ID (use ~ for negative IDs)"`
	} `cmd:"" help:"List available modes"`

	Disable struct {
		ChatID ChatID `arg:"" name:"chat_id" help:"Chat ID (use ~ for negative IDs)"`
		Type   string `arg:"" name:"type" help:"Mode type"`
	} `cmd:"" help:"Disable a mode"`

	Enable struct {
		ChatID ChatID `arg:"" name:"chat_id" help:"Chat ID (use ~ for negative IDs)"`
		Type   string `arg:"" name:"type" help:"Mode type"`
	} `cmd:"" help:"Enable a mode"`

	Delete struct {
		ChatID ChatID `arg:"" name:"chat_id" help:"Chat ID (use ~ for negative IDs)"`
		Type   string `arg:"" name:"type" help:"Mode type"`
	} `cmd:"" help:"Delete a mode"`

	Edit struct {
		ChatID ChatID `arg:"" name:"chat_id" help:"Chat ID (use ~ for negative IDs)"`
		Type   string `arg:"" name:"type" help:"Mode type"`
		Prompt string `arg:"" name:"prompt" help:"New mode prompt"`
	} `cmd:"" help:"Edit mode prompt"`
}

type UsersCmd struct {
	Remove struct {
		Username string `arg:"" help:"Username (with @ prefix)"`
	} `cmd:"" help:"Remove a user"`

	Edit struct {
		Username string   `arg:"" help:"Username (with @ prefix)"`
		Rights   []string `arg:"" help:"User rights (comma-separated)"`
	} `cmd:"" help:"Edit user rights"`

	Disable struct {
		Username string `arg:"" help:"Username (with @ prefix)"`
	} `cmd:"" help:"Disable a user"`

	Enable struct {
		Username string `arg:"" help:"Username (with @ prefix)"`
	} `cmd:"" help:"Enable a user"`

	Window struct {
		Username string `arg:"" help:"Username (with @ prefix)"`
		Limit    int64  `arg:"" help:"Window limit"`
	} `cmd:"" help:"Set user window limit"`

	Stack struct {
		Username string `arg:"" help:"Username (with @ prefix)"`
		Enabled  bool   `arg:"" help:"Enable/disable stack"`
	} `cmd:"" help:"Set user stack allowance"`
}

type DonationsCmd struct {
	Add struct {
		Username string  `arg:"" help:"Username (with @ prefix)"`
		Sum      float64 `arg:"" help:"Donation amount"`
	} `cmd:"" help:"Add a new donation"`
}

type ContextCmd struct {
	Refresh struct {
		ChatID *ChatID `arg:"" name:"chat_id" help:"Chat ID to refresh context (use ~ for negative IDs). Optional - defaults to current chat" optional:""`
	} `cmd:"" help:"Refresh chat context by marking all messages as removed"`
}