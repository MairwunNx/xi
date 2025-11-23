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

type BroadcastRepository struct{}

func NewBroadcastRepository() *BroadcastRepository {
	return &BroadcastRepository{}
}

func (x *BroadcastRepository) CreateBroadcast(logger *tracing.Logger, userID uuid.UUID, text string) (*entities.Broadcast, error) {
	defer tracing.ProfilePoint(logger, "Broadcast create broadcast completed", "repository.broadcast.create.broadcast", "user_id", userID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	broadcast := &entities.Broadcast{
		UserID: userID,
		Text:   text,
	}

	q := query.Q.WithContext(ctx)
	err := q.Broadcast.Create(broadcast)
	if err != nil {
		logger.E("Failed to create broadcast", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Broadcast created", "broadcast_id", broadcast.ID)
	return broadcast, nil
}