package metrics

import (
	"time"
	"ximanager/sources/tracing"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricsService struct {
	log *tracing.Logger
}

var (
	messagesHandled = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_messages_handled_total",
			Help: "Total number of messages handled by the poller",
		},
		[]string{"status"},
	)

	messagesIgnored = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_messages_ignored_total",
			Help: "Total number of messages ignored",
		},
		[]string{"reason"},
	)

	commandsUsed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_commands_used_total",
			Help: "Total number of commands used",
		},
		[]string{"command"},
	)

	messagesSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_messages_sent_total",
			Help: "Total number of messages sent by the diplomat",
		},
		[]string{"status"},
	)

	tokenUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_token_usage_total",
			Help: "Total number of tokens used",
		},
		[]string{"model", "type"},
	)

	costUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_cost_usage_total",
			Help: "Total cost incurred",
		},
		[]string{"model", "type"},
	)

	aiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ximanager_ai_request_duration_seconds",
			Help:    "Duration of AI provider requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model"},
	)

	messageProcessingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ximanager_message_processing_duration_seconds",
			Help:    "Total duration of message processing",
			Buckets: prometheus.DefBuckets,
		},
	)

	languagesDetected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_languages_detected_total",
			Help: "Total number of languages detected",
		},
		[]string{"lang"},
	)

	agentsUsed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_agents_used_total",
			Help: "Total number of agent executions",
		},
		[]string{"agent_name"},
	)

	statsTotalUsers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ximanager_stats_total_users",
			Help: "Total number of users",
		},
	)

	statsTotalChats = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ximanager_stats_total_chats",
			Help: "Total number of unique chats",
		},
	)

	statsTotalQuestions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ximanager_stats_total_questions",
			Help: "Total number of user questions",
		},
	)

	statsTotalCost = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ximanager_stats_total_cost",
			Help: "Total cost recorded in usage",
		},
	)

	statsTotalTokens = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ximanager_stats_total_tokens",
			Help: "Total tokens recorded in usage",
		},
	)

	statsDAU = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ximanager_stats_dau",
			Help: "Daily Active Users (last 24h)",
		},
	)

	statsMAU = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "ximanager_stats_mau",
			Help: "Monthly Active Users (last 30d)",
		},
	)

	feedbacksReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_feedbacks_received_total",
			Help: "Total number of feedbacks received",
		},
		[]string{"type"},
	)

	personalizationExtracted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ximanager_personalization_extracted_total",
			Help: "Total number of personalizations extracted",
		},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(messagesHandled)
	prometheus.MustRegister(messagesIgnored)
	prometheus.MustRegister(commandsUsed)
	prometheus.MustRegister(messagesSent)
	prometheus.MustRegister(tokenUsage)
	prometheus.MustRegister(costUsage)
	prometheus.MustRegister(languagesDetected)
	prometheus.MustRegister(agentsUsed)
	prometheus.MustRegister(statsTotalUsers)
	prometheus.MustRegister(statsTotalChats)
	prometheus.MustRegister(statsTotalQuestions)
	prometheus.MustRegister(statsTotalCost)
	prometheus.MustRegister(statsTotalTokens)
	prometheus.MustRegister(aiRequestDuration)
	prometheus.MustRegister(messageProcessingDuration)
	prometheus.MustRegister(statsDAU)
	prometheus.MustRegister(statsMAU)
	prometheus.MustRegister(feedbacksReceived)
	prometheus.MustRegister(personalizationExtracted)
}

func NewMetricsService(log *tracing.Logger) *MetricsService {
	return &MetricsService{
		log: log,
	}
}

func (s *MetricsService) RecordMessageHandled(status string) {
	messagesHandled.WithLabelValues(status).Inc()
}

func (s *MetricsService) RecordMessageIgnored(reason string) {
	messagesIgnored.WithLabelValues(reason).Inc()
}

func (s *MetricsService) RecordCommandUsed(command string) {
	commandsUsed.WithLabelValues(command).Inc()
}

func (s *MetricsService) RecordMessageSent(status string) {
	messagesSent.WithLabelValues(status).Inc()
}

func (s *MetricsService) RecordUsage(tokens int, cost float64, model string, usageType string) {
	tokenUsage.WithLabelValues(model, usageType).Add(float64(tokens))
	costUsage.WithLabelValues(model, usageType).Add(cost)
}

func (s *MetricsService) RecordLanguageDetected(lang string) {
	languagesDetected.WithLabelValues(lang).Inc()
}

func (s *MetricsService) RecordAgentUsage(agentName string) {
	agentsUsed.WithLabelValues(agentName).Inc()
}

func (s *MetricsService) RecordDialerUsage(tokens int, cost float64, model string) {
	s.RecordUsage(tokens, cost, model, "dialer")
}

func (s *MetricsService) RecordAgentCost(tokens int, cost float64, model string) {
	s.RecordUsage(tokens, cost, model, "agent")
}

func (s *MetricsService) RecordAIRequestDuration(duration time.Duration, model string) {
	aiRequestDuration.WithLabelValues(model).Observe(duration.Seconds())
}

func (s *MetricsService) RecordMessageProcessingDuration(duration time.Duration) {
	messageProcessingDuration.Observe(duration.Seconds())
}

func (s *MetricsService) SetTotalUsers(count float64) {
	statsTotalUsers.Set(count)
}

func (s *MetricsService) SetTotalChats(count float64) {
	statsTotalChats.Set(count)
}

func (s *MetricsService) SetTotalQuestions(count float64) {
	statsTotalQuestions.Set(count)
}

func (s *MetricsService) SetTotalCost(cost float64) {
	statsTotalCost.Set(cost)
}

func (s *MetricsService) SetTotalTokens(tokens float64) {
	statsTotalTokens.Set(tokens)
}

func (s *MetricsService) SetDAU(count float64) {
	statsDAU.Set(count)
}

func (s *MetricsService) SetMAU(count float64) {
	statsMAU.Set(count)
}

func (s *MetricsService) RecordFeedback(feedbackType string) {
	feedbacksReceived.WithLabelValues(feedbackType).Inc()
}

func (s *MetricsService) RecordPersonalizationExtracted(status string) {
	personalizationExtracted.WithLabelValues(status).Inc()
}