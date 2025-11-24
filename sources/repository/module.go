package repository

import "go.uber.org/fx"

var Module = fx.Module("repository",
	fx.Provide(
		NewTariffsRepository,
		NewModesRepository,
		NewUsersRepository,
		NewMessagesRepository,
		NewRightsRepository,
		NewDonationsRepository,
		NewPersonalizationsRepository,
		NewUsageRepository,
		NewHealthRepository,
		NewBansRepository,
		NewBroadcastRepository,
		NewFeedbacksRepository,
	),
)