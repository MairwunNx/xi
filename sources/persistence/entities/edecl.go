package entities

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type (
	Donation struct {
		ID        uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
		User      uuid.UUID       `gorm:"type:uuid;not null;column:user" json:"user"`
		Sum       decimal.Decimal `gorm:"type:decimal(10,2);not null" json:"sum"`
		CreatedAt time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`

		UserEntity User `gorm:"foreignKey:User;references:ID" json:"user_entity"`
	}

	Message struct {
		ID           uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
		ChatID       int64         `gorm:"not null" json:"chat_id"`
		MessageTime  time.Time     `gorm:"default:CURRENT_TIMESTAMP" json:"message_time"`
		MessageText  EncryptedText `gorm:"type:bytea;not null" json:"message_text"`
		IsAggressive bool          `gorm:"not null" json:"is_aggressive"`
		IsXiResponse bool          `gorm:"not null" json:"is_xi_response"`
		IsRemoved    bool          `gorm:"not null;default:false" json:"is_removed"`
		UserID       *uuid.UUID    `gorm:"type:uuid" json:"user_id"`

		User *User `gorm:"foreignKey:UserID;references:ID" json:"user"`
	}

	Mode struct {
		ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
		ChatID    int64      `gorm:"not null" json:"chat_id"`
		Type      string     `gorm:"size:50;not null" json:"type"`
		Name      string     `gorm:"size:255;not null" json:"name"`
		Prompt    string     `gorm:"type:text;not null" json:"prompt"`
		Config    *string    `gorm:"type:json;column:config" json:"config"`
		Final     *bool       `gorm:"not null;default:false" json:"final"`
		IsEnabled *bool       `gorm:"not null;default:true" json:"is_enabled"`
		CreatedAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
		CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by"`

		Creator       *User          `gorm:"foreignKey:CreatedBy;references:ID" json:"creator"`
		SelectedModes []SelectedMode `gorm:"foreignKey:ModeID;references:ID" json:"selected_modes"`
	}

	SelectedMode struct {
		ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
		ChatID     int64     `gorm:"not null" json:"chat_id"`
		ModeID     uuid.UUID `gorm:"type:uuid;not null" json:"mode_id"`
		SwitchedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"switched_at"`
		SwitchedBy uuid.UUID `gorm:"type:uuid;not null" json:"switched_by"`

		Mode Mode `gorm:"foreignKey:ModeID;references:ID" json:"mode"`
		User User `gorm:"foreignKey:SwitchedBy;references:ID" json:"user"`
	}

	Pin struct {
		ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
		ChatID    int64     `gorm:"not null" json:"chat_id"`
		User      uuid.UUID `gorm:"type:uuid;not null;column:user" json:"user"`
		Message   string    `gorm:"type:text;not null" json:"message"`
		CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`

		UserEntity User `gorm:"foreignKey:User;references:ID" json:"user_entity"`
	}

	User struct {
		ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
		UserID         int64          `gorm:"uniqueIndex;not null" json:"user_id"`
		Username       *string        `gorm:"size:255" json:"username"`
		Fullname       *string        `gorm:"size:255" json:"fullname"`
		Rights         pq.StringArray `gorm:"type:user_right[];not null;default:ARRAY[]::user_right[]" json:"rights"`
		IsActive       *bool          `gorm:"not null;default:true" json:"is_active"`
		IsStackAllowed *bool          `gorm:"not null;default:false" json:"is_stack_allowed"`
		IsStackEnabled *bool          `gorm:"not null;default:true" json:"is_stack_enabled"`
		WindowLimit    int64          `gorm:"not null;default:0" json:"window_limit"`
		CreatedAt      time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`

		Messages      []Message      `gorm:"foreignKey:UserID;references:ID" json:"messages"`
		Donations     []Donation     `gorm:"foreignKey:User;references:ID" json:"donations"`
		CreatedModes  []Mode         `gorm:"foreignKey:CreatedBy;references:ID" json:"created_modes"`
		SelectedModes []SelectedMode `gorm:"foreignKey:SwitchedBy;references:ID" json:"selected_modes"`
		Pins          []Pin          `gorm:"foreignKey:User;references:ID" json:"pins"`
	}
)

func (Donation) TableName() string     { return "xi_donations" }
func (Message) TableName() string      { return "xi_messages" }
func (Mode) TableName() string         { return "xi_modes" }
func (Pin) TableName() string          { return "xi_pins" }
func (SelectedMode) TableName() string { return "xi_selected_modes" }
func (User) TableName() string         { return "xi_users" }
