package artificial

import (
	"context"
	"fmt"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
)

func getTariffWithFallback(
	ctx context.Context,
	tariffs repository.Tariffs,
	userGrade platform.UserGrade,
) (*entities.Tariff, error) {
	tariff, err := tariffs.GetLatestByKey(ctx, string(userGrade))
	if err != nil {
		// Fallback to bronze if not bronze already
		if userGrade != platform.GradeBronze {
			tariff, err = tariffs.GetLatestByKey(ctx, string(platform.GradeBronze))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tariff for %s (with bronze fallback): %w", userGrade, err)
		}
	}
	return tariff, nil
}