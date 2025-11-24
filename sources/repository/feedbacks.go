package repository

import (
	"context"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/google/uuid"
)

type FeedbacksRepository struct{}

func NewFeedbacksRepository() *FeedbacksRepository {
	return &FeedbacksRepository{}
}

func (x *FeedbacksRepository) CreateFeedback(logger *tracing.Logger, userID uuid.UUID, liked int) (*entities.Feedback, error) {
	defer tracing.ProfilePoint(logger, "Feedbacks create feedback completed", "repository.feedbacks.create", "user_id", userID, "liked", liked)()

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	feedback := &entities.Feedback{
		UserID: userID,
		Liked:  liked,
	}

	if err := q.Feedback.Create(feedback); err != nil {
		logger.E("Failed to create feedback", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Feedback created successfully", "feedback_id", feedback.ID, "user_id", userID, "liked", liked)
	return feedback, nil
}

func (x *FeedbacksRepository) GetFeedbackStats(logger *tracing.Logger) (likes int64, dislikes int64, err error) {
	defer tracing.ProfilePoint(logger, "Feedbacks get stats completed", "repository.feedbacks.get.stats")()

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	likes, err = q.Feedback.Where(query.Feedback.Liked.Eq(1)).Count()
	if err != nil {
		logger.E("Failed to count likes", tracing.InnerError, err)
		return 0, 0, err
	}

	dislikes, err = q.Feedback.Where(query.Feedback.Liked.Eq(0)).Count()
	if err != nil {
		logger.E("Failed to count dislikes", tracing.InnerError, err)
		return 0, 0, err
	}

	return likes, dislikes, nil
}

func (x *FeedbacksRepository) GetUserFeedbackStats(logger *tracing.Logger, userID uuid.UUID) (likes int64, dislikes int64, err error) {
	defer tracing.ProfilePoint(logger, "Feedbacks get user stats completed", "repository.feedbacks.get.user.stats", "user_id", userID)()

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	likes, err = q.Feedback.Where(query.Feedback.UserID.Eq(userID), query.Feedback.Liked.Eq(1)).Count()
	if err != nil {
		logger.E("Failed to count user likes", tracing.InnerError, err)
		return 0, 0, err
	}

	dislikes, err = q.Feedback.Where(query.Feedback.UserID.Eq(userID), query.Feedback.Liked.Eq(0)).Count()
	if err != nil {
		logger.E("Failed to count user dislikes", tracing.InnerError, err)
		return 0, 0, err
	}

	return likes, dislikes, nil
}