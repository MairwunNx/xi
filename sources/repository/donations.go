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

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	donations, err := q.Donation.Where(query.Donation.User.Eq(user.ID)).Order(query.Donation.CreatedAt.Desc()).Find()
	if err != nil {
		logger.E("Failed to get donations", tracing.InnerError, err)
		logger.I("User grade fallback inferred", "internal_user_grade", platform.GradeBronze)
		return platform.GradeBronze, err
	}

	if len(donations) == 0 {
		logger.I("User grade inferred", "internal_user_grade", platform.GradeBronze)
		return platform.GradeBronze, nil
	}

	if donations[0].CreatedAt.After(time.Now().AddDate(0, 0, -30)) {
		logger.I("User grade inferred", "internal_user_grade", platform.GradeGold)
		return platform.GradeGold, nil
	}

	logger.I("User grade inferred", "internal_user_grade", platform.GradeSilver)
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