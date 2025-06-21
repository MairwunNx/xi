package persistence

import (
	"fmt"
	"time"

	"ximanager/sources/tracing"

	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgresDatabase(lc fx.Lifecycle, config *DatabaseConfig, log *tracing.Logger) *gorm.DB {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode, config.TimeZone,
	)

	gormlogger := logger.New(
		&gormtracer{logger: log},
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger})
	if err != nil {
		log.F("Failed to connect to database", tracing.InnerError, err)
	}

	sqldb, err := db.DB()
	if err != nil {
		log.F("Failed to get underlying sql.DB", tracing.InnerError, err)
	}

	sqldb.SetMaxOpenConns(10)
	sqldb.SetMaxIdleConns(2)
	sqldb.SetConnMaxLifetime(2 * time.Hour)
	sqldb.SetConnMaxIdleTime(30 * time.Minute)

	log.I("Database initialized successfully")

	return db
}
