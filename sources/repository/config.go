package repository

import (
	"ximanager/sources/platform"
)

type PinsConfig struct {
	PinsLimitDefault int
	PinsLimitDonated int
	PinsLimitChat    int
}

func NewPinsConfig() *PinsConfig {
	return &PinsConfig{
		PinsLimitDefault: platform.GetAsInt("PINS_LIMIT_DEFAULT", 3),
		PinsLimitDonated: platform.GetAsInt("PINS_LIMIT_DONATED", 10),
		PinsLimitChat:    platform.GetAsInt("PINS_LIMIT_CHAT", 16),
	}
}
