package repository

import (
	"context"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/shopspring/decimal"
)

type DonationsRepository struct{}

func NewDonationsRepository() *DonationsRepository {
	return &DonationsRepository{}
}

func (x *DonationsRepository) GetUserGrade(logger *tracing.Logger, user *entities.User) (platform.UserGrade, error) {
	if strings.HasSuffix(*user.Username, "ximanager") {
		return platform.GradeSilver, nil
	}

	// ????????? ?????????? ??? ????? ?????????????
	now := time.Now()
	userAge := now.Sub(user.CreatedAt)
	
	// ?????? 3 ??? - Gold Grade
	if userAge <= 3*24*time.Hour {
		logger.I("User grade inferred (loyalty: first 3 days)", "internal_user_grade", platform.GradeGold, "user_age_hours", userAge.Hours())
		return platform.GradeGold, nil
	}
	
	// ????????? 3 ??? (4-6 ????) - Silver Grade
	if userAge <= 6*24*time.Hour {
		logger.I("User grade inferred (loyalty: days 4-6)", "internal_user_grade", platform.GradeSilver, "user_age_hours", userAge.Hours())
		return platform.GradeSilver, nil
	}

	// ????? 6 ???? ?????????? ???????????? ?????? ? ????????
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

func (x *DonationsRepository) GetDonationsByUser(logger *tracing.Logger, user *entities.User) ([]*entities.Donation, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	donations, err := q.Donation.Where(query.Donation.User.Eq(user.ID)).Order(query.Donation.CreatedAt.Desc()).Find()
	if err != nil {
		logger.E("Failed to get donations", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Donations fetched")
	return donations, nil
}

func (x *DonationsRepository) GetDonationsWithUsers(logger *tracing.Logger) ([]*entities.Donation, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	donations, err := q.Donation.Preload(query.Donation.UserEntity).Order(query.Donation.Sum.Desc()).Find()
	if err != nil {
		logger.E("Failed to get donations with users", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Donations with users fetched")
	return donations, nil
}

func (x *DonationsRepository) CreateDonation(logger *tracing.Logger, user *entities.User, sum float64) (*entities.Donation, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	donation := &entities.Donation{
		User: user.ID,
		Sum:  decimal.NewFromFloat(sum),
	}

	err := q.Donation.Create(donation)
	if err != nil {
		logger.E("Failed to create donation", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Created donation")
	return donation, nil
}