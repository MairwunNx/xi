package repository

import (
	"context"
	"fmt"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/tracing"
)

type Tariffs interface {
	GetLatestByKey(ctx context.Context, key string) (*entities.Tariff, error)
}

type tariffs struct {
	log *tracing.Logger
}

func NewTariffsRepository(log *tracing.Logger) Tariffs {
	return &tariffs{log: log}
}

func (x *tariffs) GetLatestByKey(ctx context.Context, key string) (*entities.Tariff, error) {
	defer tracing.ProfilePoint(x.log, "Tariffs get latest by key completed", "repository.tariffs.get.latest.by.key", "key", key)()
  t := query.Q.Tariff
	
	tariff, err := t.WithContext(ctx).
		Where(t.Key.Eq(key)).
		Order(t.CreatedAt.Desc()).
		First()
		
	if err != nil {
		return nil, fmt.Errorf("failed to get latest tariff for key %s: %w", key, err)
	}
	
	return tariff, nil
}