package repository

import (
	"context"
	"errors"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"gorm.io/gorm"
)

var (
	ErrPersonalizationNotFound = errors.New("personalization not found")
	ErrPersonalizationTooShort = errors.New("personalization prompt too short")
	ErrPersonalizationTooLong  = errors.New("personalization prompt too long")
)

type PersonalizationsRepository struct{}

func NewPersonalizationsRepository() *PersonalizationsRepository {
	return &PersonalizationsRepository{}
}

func (x *PersonalizationsRepository) CreateOrUpdatePersonalization(logger *tracing.Logger, user *entities.User, prompt string) (*entities.Personalization, error) {
	defer tracing.ProfilePoint(logger, "Personalizations create or update completed", "repository.personalizations.create.or.update", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	prompt = strings.TrimSpace(strings.ToValidUTF8(prompt, ""))

	promptRunes := []rune(prompt)
	if len(promptRunes) < 12 {
		return nil, ErrPersonalizationTooShort
	}

	if len(promptRunes) > 1024 {
		return nil, ErrPersonalizationTooLong
	}

	q := query.Q.WithContext(ctx)

	existing, err := q.Personalization.Where(query.Personalization.UserID.Eq(user.ID)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.E("Failed to check existing personalization", tracing.InnerError, err)
		return nil, err
	}

	if existing != nil {
		existing.Prompt = prompt
		existing.UpdatedAt = time.Now()
		err = q.Personalization.Save(existing)
		if err != nil {
			logger.E("Failed to update personalization", tracing.InnerError, err)
			return nil, err
		}
		logger.I("Updated personalization")
		return existing, nil
	}

	personalization := &entities.Personalization{
		UserID:    user.ID,
		Prompt:    prompt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = q.Personalization.Create(personalization)
	if err != nil {
		logger.E("Failed to create personalization", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Created personalization")
	return personalization, nil
}

func (x *PersonalizationsRepository) GetPersonalizationByUser(logger *tracing.Logger, user *entities.User) (*entities.Personalization, error) {
	defer tracing.ProfilePoint(logger, "Personalizations get personalization by user completed", "repository.personalizations.get.personalization.by.user", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	personalization, err := q.Personalization.Where(query.Personalization.UserID.Eq(user.ID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.W("Personalization not found")
			return nil, ErrPersonalizationNotFound
		} else {
			logger.E("Failed to get personalization by user", tracing.InnerError, err)
			return nil, err
		}
	}

	logger.I("Personalization fetched by user")
	return personalization, nil
}

func (x *PersonalizationsRepository) DeletePersonalization(logger *tracing.Logger, user *entities.User) error {
	defer tracing.ProfilePoint(logger, "Personalizations delete personalization completed", "repository.personalizations.delete.personalization", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	result, err := q.Personalization.Where(query.Personalization.UserID.Eq(user.ID)).Delete(&entities.Personalization{})
	if err != nil {
		logger.E("Failed to delete personalization", tracing.InnerError, err)
		return err
	}

	if result.RowsAffected == 0 {
		return ErrPersonalizationNotFound
	}

	logger.I("Personalization deleted")
	return nil
}