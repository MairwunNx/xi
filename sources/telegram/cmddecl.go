package telegram

type ChatID int64

type ModeCmd struct {
	Create struct {
	} `cmd:"" help:"Start interactive mode creation"`

	Edit struct {
		Type string `arg:"" name:"type" help:"Mode type key to edit"`
	} `cmd:"" help:"Start interactive mode editing"`

	Info struct {
	} `cmd:"" help:"Show mode information"`
}

type UsersCmd struct {
	Username string `arg:"" optional:"" help:"Username (with @ prefix)"`
}

type PersonalizationCmd struct {
	Help struct {
	} `cmd:"" help:"Show personalization help"`
}

type ContextCmd struct {
	Help struct {
	} `cmd:"" help:"Show context management help"`
}

type BanCmd struct {
	Username string `arg:"" help:"Username to ban (with or without @ prefix)"`
	Reason   string `arg:"" help:"Reason for ban"`
	Duration string `arg:"" help:"Ban duration (e.g., 1d, 4h, 10m, 60s)"`
}

type PardonCmd struct {
	Username string `arg:"" help:"Username to unban (with or without @ prefix)"`
}

type BroadcastCmd struct {
	Help struct {
	} `cmd:"" help:"Show broadcast help"`
}

type TariffCmd struct {
	Add struct {
		Key    string `arg:"" help:"Tariff key (e.g., bronze, silver, gold)"`
		Config string `arg:"" help:"Tariff configuration (JSON)"`
	} `cmd:"" help:"Add a new tariff"`

	List struct {
	} `cmd:"" help:"List all tariffs"`

	Get struct {
		Key string `arg:"" help:"Tariff key to get"`
	} `cmd:"" help:"Get tariff by key"`
}