package repository

import "go.uber.org/fx"

var Module = fx.Module("repository",
	fx.Provide(
		NewPinsConfig,
		NewModesRepository,
		NewUsersRepository,
		NewMessagesRepository,
		NewRightsRepository,
		NewDonationsRepository,
		NewPinsRepository,
		NewUsageRepository,
		NewHealthRepository,
		NewBansRepository,
	),
)