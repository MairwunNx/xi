package repository

import "go.uber.org/fx"

var Module = fx.Module("repository",
	fx.Provide(
		NewModesRepository,
		NewUsersRepository,
		NewMessagesRepository,
		NewRightsRepository,
		NewDonationsRepository,
		NewPersonalizationsRepository,
		NewUsageRepository,
		NewHealthRepository,
		NewBansRepository,
		NewTariffsRepository,
	),
)