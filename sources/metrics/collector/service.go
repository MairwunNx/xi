package collector

import (
	"context"
	"time"
	"ximanager/sources/metrics"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	"go.uber.org/fx"
)

type StatsCollector struct {
	log       *tracing.Logger
	metrics   *metrics.MetricsService
	messages  *repository.MessagesRepository
	users     *repository.UsersRepository
	usage     *repository.UsageRepository
	donations *repository.DonationsRepository
}

func NewStatsCollector(
	lc fx.Lifecycle,
	log *tracing.Logger,
	metrics *metrics.MetricsService,
	messages *repository.MessagesRepository,
	users *repository.UsersRepository,
	usage *repository.UsageRepository,
	donations *repository.DonationsRepository,
) *StatsCollector {
	s := &StatsCollector{
		log:       log,
		metrics:   metrics,
		messages:  messages,
		users:     users,
		usage:     usage,
		donations: donations,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go s.start()
			return nil
		},
	})

	return s
}

func (s *StatsCollector) start() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.collectStats()

	for range ticker.C {
		s.collectStats()
	}
}

func (s *StatsCollector) collectStats() {
	if count, err := s.users.GetTotalUsersCount(s.log); err == nil {
		s.metrics.SetTotalUsers(float64(count))
	} else {
		s.log.E("Failed to collect total users stats", tracing.InnerError, err)
	}

	if count, err := s.messages.GetUniqueChatCount(s.log); err == nil {
		s.metrics.SetTotalChats(float64(count))
	} else {
		s.log.E("Failed to collect total chats stats", tracing.InnerError, err)
	}

	if count, err := s.messages.GetTotalUserQuestionsCount(s.log); err == nil {
		s.metrics.SetTotalQuestions(float64(count))
	} else {
		s.log.E("Failed to collect total questions stats", tracing.InnerError, err)
	}

	if cost, err := s.usage.GetTotalCost(s.log); err == nil {
		s.metrics.SetTotalCost(cost.InexactFloat64())
	} else {
		s.log.E("Failed to collect total cost stats", tracing.InnerError, err)
	}

	if tokens, err := s.usage.GetTotalTokens(s.log); err == nil {
		s.metrics.SetTotalTokens(float64(tokens))
	} else {
		s.log.E("Failed to collect total tokens stats", tracing.InnerError, err)
	}

	if count, err := s.usage.GetActiveUsersCount(s.log, time.Now().Add(-24*time.Hour)); err == nil {
		s.metrics.SetDAU(float64(count))
	} else {
		s.log.E("Failed to collect DAU stats", tracing.InnerError, err)
	}

	if count, err := s.usage.GetActiveUsersCount(s.log, time.Now().Add(-30*24*time.Hour)); err == nil {
		s.metrics.SetMAU(float64(count))
	} else {
		s.log.E("Failed to collect MAU stats", tracing.InnerError, err)
	}
}