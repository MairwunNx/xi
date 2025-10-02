package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/google/uuid"
)

var (
	ErrBanNotFound       = errors.New("ban not found")
	ErrInvalidDuration   = errors.New("invalid duration format")
	ErrBanActive         = errors.New("user has active ban")
	ErrDurationTooLong   = errors.New("ban duration exceeds maximum of 12 hours")
	ErrDurationTooShort  = errors.New("ban duration is too short")
)

type BansRepository struct{}

func NewBansRepository() *BansRepository {
	return &BansRepository{}
}

// ParseDuration парсит строку длительности в формате "1d", "4h", "10m", "60s" и возвращает time.Duration
func (x *BansRepository) ParseDuration(durationStr string) (time.Duration, error) {
	durationStr = strings.TrimSpace(durationStr)
	if durationStr == "" {
		return 0, ErrInvalidDuration
	}

	// Извлекаем число и единицу измерения
	var value int64
	var unit string
	
	for i, ch := range durationStr {
		if ch < '0' || ch > '9' {
			numStr := durationStr[:i]
			unit = strings.ToLower(durationStr[i:])
			
			var err error
			value, err = strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, ErrInvalidDuration
			}
			break
		}
	}

	if value == 0 {
		return 0, ErrInvalidDuration
	}

	var duration time.Duration
	switch unit {
	case "s", "sec", "second", "seconds":
		duration = time.Duration(value) * time.Second
	case "m", "min", "minute", "minutes":
		duration = time.Duration(value) * time.Minute
	case "h", "hr", "hour", "hours":
		duration = time.Duration(value) * time.Hour
	case "d", "day", "days":
		duration = time.Duration(value) * 24 * time.Hour
	default:
		return 0, ErrInvalidDuration
	}

	// Проверяем минимальную длительность (0 секунд)
	if duration < 0 {
		return 0, ErrDurationTooShort
	}

	// Проверяем максимальную длительность (12 часов)
	maxDuration := 12 * time.Hour
	if duration > maxDuration {
		return 0, ErrDurationTooLong
	}

	return duration, nil
}

// CreateBan создает новую запись бана
func (x *BansRepository) CreateBan(logger *tracing.Logger, userID uuid.UUID, chatID int64, reason string, duration string) (*entities.Ban, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	// Проверяем валидность длительности
	_, err := x.ParseDuration(duration)
	if err != nil {
		logger.W("Invalid ban duration", "duration", duration, tracing.InnerError, err)
		return nil, err
	}

	q := query.Q.WithContext(ctx)

	ban := &entities.Ban{
		UserID:      userID,
		Reason:      reason,
		Duration:    duration,
		BannedAt:    time.Now(),
		BannedWhere: chatID,
	}

	err = q.Ban.Create(ban)
	if err != nil {
		logger.E("Failed to create ban", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Ban created", "user_id", userID, "duration", duration, "reason", reason)
	return ban, nil
}

// GetActiveBan возвращает активный бан для пользователя, если он есть
func (x *BansRepository) GetActiveBan(logger *tracing.Logger, userID uuid.UUID) (*entities.Ban, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	// Получаем все баны пользователя, отсортированные по времени создания (новые первыми)
	bans, err := q.Ban.Where(query.Ban.UserID.Eq(userID)).Order(query.Ban.BannedAt.Desc()).Find()
	if err != nil {
		logger.E("Failed to get user bans", tracing.InnerError, err)
		return nil, err
	}

	if len(bans) == 0 {
		return nil, ErrBanNotFound
	}

	// Проверяем каждый бан на активность
	for _, ban := range bans {
		duration, err := x.ParseDuration(ban.Duration)
		if err != nil {
			logger.W("Failed to parse ban duration, skipping", "ban_id", ban.ID, tracing.InnerError, err)
			continue
		}

		expiresAt := ban.BannedAt.Add(duration)
		if time.Now().Before(expiresAt) {
			logger.I("Active ban found", "ban_id", ban.ID, "expires_at", expiresAt)
			return ban, nil
		}
	}

	return nil, ErrBanNotFound
}

// GetActiveBanWithExpiry возвращает активный бан и время его истечения
func (x *BansRepository) GetActiveBanWithExpiry(logger *tracing.Logger, userID uuid.UUID) (*entities.Ban, time.Time, error) {
	ban, err := x.GetActiveBan(logger, userID)
	if err != nil {
		return nil, time.Time{}, err
	}

	duration, err := x.ParseDuration(ban.Duration)
	if err != nil {
		logger.E("Failed to parse ban duration", tracing.InnerError, err)
		return nil, time.Time{}, err
	}

	expiresAt := ban.BannedAt.Add(duration)
	return ban, expiresAt, nil
}

// IsUserBanned проверяет, заблокирован ли пользователь в данный момент
func (x *BansRepository) IsUserBanned(logger *tracing.Logger, userID uuid.UUID) bool {
	_, err := x.GetActiveBan(logger, userID)
	return err == nil
}

// DeleteBansByUser удаляет все баны пользователя (разбанивание)
func (x *BansRepository) DeleteBansByUser(logger *tracing.Logger, userID uuid.UUID) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	_, err := q.Ban.Where(query.Ban.UserID.Eq(userID)).Delete()
	if err != nil {
		logger.E("Failed to delete user bans", tracing.InnerError, err)
		return err
	}

	logger.I("User bans deleted", "user_id", userID)
	return nil
}

// GetBansByUser возвращает все баны пользователя
func (x *BansRepository) GetBansByUser(logger *tracing.Logger, userID uuid.UUID) ([]*entities.Ban, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	bans, err := q.Ban.Where(query.Ban.UserID.Eq(userID)).Order(query.Ban.BannedAt.Desc()).Find()
	if err != nil {
		logger.E("Failed to get user bans", tracing.InnerError, err)
		return nil, err
	}

	return bans, nil
}

// FormatBanExpiry форматирует время окончания бана в читаемый вид
func (x *BansRepository) FormatBanExpiry(expiresAt time.Time) string {
	moscowTime := expiresAt.UTC().Add(3 * time.Hour)
	return moscowTime.Format("02.01.2006 в 15:04")
}

// GetRemainingDuration возвращает оставшееся время бана
func (x *BansRepository) GetRemainingDuration(expiresAt time.Time) time.Duration {
	remaining := time.Until(expiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// FormatRemainingTime форматирует оставшееся время в читаемый вид
func (x *BansRepository) FormatRemainingTime(duration time.Duration) string {
	if duration <= 0 {
		return "истек"
	}

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	var parts []string
	
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d ч", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%d мин", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d сек", seconds))
	}

	return strings.Join(parts, " ")
}
