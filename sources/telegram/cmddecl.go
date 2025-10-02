package telegram

type ChatID int64

type ModeCmd struct {
	Add struct {
		ChatID ChatID `arg:"" name:"chat_id" help:"Chat ID (use ~ for negative IDs)"`
		Type   string `arg:"" name:"type" help:"Mode type"`
		Name   string `arg:"" name:"name" help:"Mode name"`
		Config string `arg:"" name:"config" help:"Mode configuration (JSON)"`
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
		Config string `arg:"" name:"config" help:"Mode configuration (JSON)"`
	} `cmd:"" help:"Edit mode configuration"`
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
		Action   string `arg:"" enum:"enable,disable,true,false,1,0" help:"Enable or disable stack access (enable/disable/true/false/1/0)"`
	} `cmd:"" help:"Set user stack allowance"`
}

type DonationsCmd struct {
	Add struct {
		Username string  `arg:"" help:"Username (with @ prefix)"`
		Sum      float64 `arg:"" help:"Donation amount"`
	} `cmd:"" help:"Add a new donation"`

	List struct {
	} `cmd:"" help:"List all donations"`
}

type PinnedCmd struct {
	Add struct {
		Message string `arg:"" help:"Message to pin (max 1024 characters)"`
	} `cmd:"" help:"Add a new pinned message"`

	Remove struct {
		Message string `arg:"" help:"Message to remove from pins"`
	} `cmd:"" help:"Remove a pinned message"`

	List struct {
	} `cmd:"" help:"List all pinned messages"`
}

type ContextCmd struct {
	Refresh struct {
	} `cmd:"" help:"Clear context memory for current chat"`

	Help struct {
	} `cmd:"" help:"Show context management help"`
}

type BanCmd struct {
	UserID   int64  `arg:"" help:"User ID to ban"`
	Reason   string `arg:"" help:"Reason for ban"`
	Duration string `arg:"" help:"Ban duration (e.g., 1d, 4h, 10m, 60s)"`
}

type PardonCmd struct {
	UserID int64 `arg:"" help:"User ID to unban"`
}