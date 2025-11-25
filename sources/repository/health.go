package repository

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"
	openrouter "github.com/revrost/go-openrouter"
)

type HealthRepository struct {
	redis        *redis.Client
	openrouter   *openrouter.Client
	httpClient   *http.Client
	config       *configuration.Config
}

func NewHealthRepository(redis *redis.Client, openrouter *openrouter.Client, httpClient *http.Client, config *configuration.Config) *HealthRepository {
	return &HealthRepository{
		redis:      redis,
		openrouter: openrouter,
		httpClient: httpClient,
		config:     config,
	}
}

func (x *HealthRepository) CheckDatabaseHealth(logger *tracing.Logger) error {
	defer tracing.ProfilePoint(logger, "Health check database completed", "repository.health.check.database")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 1*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	_, err := q.User.Limit(1).Find()
	if err != nil {
		logger.E("Database health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("Database health check passed")
	return nil
}

func (x *HealthRepository) CheckRedisHealth(logger *tracing.Logger) error {
	defer tracing.ProfilePoint(logger, "Health check redis completed", "repository.health.check.redis")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 1*time.Second)
	defer cancel()

	err := x.redis.Ping(ctx).Err()
	if err != nil {
		logger.E("Redis health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("Redis health check passed")
	return nil
}

func (x *HealthRepository) CheckProxyHealth(logger *tracing.Logger) error {
	defer tracing.ProfilePoint(logger, "Health check proxy completed", "repository.health.check.proxy")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://httpbin.org/ip", nil)
	if err != nil {
		logger.E("Proxy health check failed: request creation error", tracing.InnerError, err)
		return err
	}

	resp, err := x.httpClient.Do(req)
	if err != nil {
		logger.E("Proxy health check failed: request error", tracing.InnerError, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("proxy health check failed: status %d", resp.StatusCode)
		logger.E("Proxy health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("Proxy health check passed")
	return nil
}

func (x *HealthRepository) CheckOpenRouterHealth(logger *tracing.Logger) error {
	defer tracing.ProfilePoint(logger, "Health check openrouter completed", "repository.health.check.openrouter")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 10*time.Second)
	defer cancel()

	_, err := x.openrouter.ListModels(ctx)
	if err != nil {
		logger.E("OpenRouter health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("OpenRouter health check passed")
	return nil
}

func (x *HealthRepository) CheckUnleashHealth(logger *tracing.Logger) error {
	defer tracing.ProfilePoint(logger, "Health check unleash completed", "repository.health.check.unleash")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	unleashURL := "http://ximanager-unleash:4242/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, unleashURL, nil)
	if err != nil {
		logger.E("Unleash health check failed: request creation error", tracing.InnerError, err)
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.E("Unleash health check failed: request error", tracing.InnerError, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unleash health check failed: status %d", resp.StatusCode)
		logger.E("Unleash health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("Unleash health check passed")
	return nil
}

func (x *HealthRepository) CheckTelegramHealth(logger *tracing.Logger, bot *tgbotapi.BotAPI) error {
	defer tracing.ProfilePoint(logger, "Health check telegram completed", "repository.health.check.telegram")()

	if bot == nil {
		err := fmt.Errorf("telegram bot is nil")
		logger.E("Telegram health check failed", tracing.InnerError, err)
		return err
	}

	_, err := bot.GetMe()
	if err != nil {
		logger.E("Telegram health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("Telegram health check passed")
	return nil
}