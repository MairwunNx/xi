package artificial

import (
	"fmt"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"
)

func getTariffWithFallback(
	log *tracing.Logger,
	tariffs *repository.TariffsRepository,
	userGrade platform.UserGrade,
) (*entities.Tariff, error) {
	tariff, err := tariffs.GetLatestByKey(log, string(userGrade))
	if err != nil {
		// Fallback to bronze if not bronze already
		if userGrade != platform.GradeBronze {
			tariff, err = tariffs.GetLatestByKey(log, string(platform.GradeBronze))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tariff for %s (with bronze fallback): %w", userGrade, err)
		}
	}
	return tariff, nil
}