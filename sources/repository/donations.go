package repository

import (
	"context"
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

func (x *DonationsRepository) MustGetDonationsByUser(logger *tracing.Logger, user *entities.User) []*entities.Donation {
	donations, err := x.GetDonationsByUser(logger, user)
	if err != nil {
		logger.F("Got error while expecting donation", tracing.InnerError, err)
	}

	return donations
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

func (x *DonationsRepository) MustCreateDonation(logger *tracing.Logger, user *entities.User, sum float64) *entities.Donation {
	donation, err := x.CreateDonation(logger, user, sum)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}

	return donation
}