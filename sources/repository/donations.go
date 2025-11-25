package repository

import (
	"context"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"
)

type DonationsRepository struct{}

func NewDonationsRepository() *DonationsRepository {
	return &DonationsRepository{}
}

func (x *DonationsRepository) GetUserGrade(logger *tracing.Logger, user *entities.User) (platform.UserGrade, error) {
	defer tracing.ProfilePoint(logger, "Donations get user grade completed", "repository.donations.get.user.grade", "user_id", user.ID)()
	if strings.HasSuffix(*user.Username, "ximanager") {
		return platform.GradeSilver, nil
	}

	now := time.Now()
	userAge := now.Sub(user.CreatedAt)

	if userAge <= 3*24*time.Hour {
		logger.I("User grade inferred (loyalty: first 3 days)", "internal_user_grade", platform.GradeGold, "user_age_hours", userAge.Hours())
		return platform.GradeGold, nil
	}

	if userAge <= 6*24*time.Hour {
		logger.I("User grade inferred (loyalty: days 4-6)", "internal_user_grade", platform.GradeSilver, "user_age_hours", userAge.Hours())
		return platform.GradeSilver, nil
	}

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	donations, err := q.Donation.Where(query.Donation.User.Eq(user.ID)).Order(query.Donation.CreatedAt.Desc()).Find()
	if err != nil {
		logger.E("Failed to get donations", tracing.InnerError, err)
		logger.I("User grade fallback inferred", "internal_user_grade", platform.GradeBronze, "user_age_hours", userAge.Hours())
		return platform.GradeBronze, err
	}

	if len(donations) == 0 {
		logger.I("User grade inferred (no donations)", "internal_user_grade", platform.GradeBronze, "user_age_hours", userAge.Hours())
		return platform.GradeBronze, nil
	}

	if donations[0].CreatedAt.After(time.Now().AddDate(0, 0, -30)) {
		logger.I("User grade inferred (recent donation)", "internal_user_grade", platform.GradeGold, "user_age_hours", userAge.Hours())
		return platform.GradeGold, nil
	}

	logger.I("User grade inferred (old donation)", "internal_user_grade", platform.GradeSilver, "user_age_hours", userAge.Hours())
	return platform.GradeSilver, nil
}