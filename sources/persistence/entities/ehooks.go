package entities

import (
	"context"
	"fmt"
	"ximanager/sources/platform"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EncryptedText string

func (EncryptedText) GormDataType() string {
	return "bytea"
}

func (et EncryptedText) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	encryptionKey := platform.Get("MESSAGE_ENCRYPTION_KEY", "")
	if encryptionKey == "" {
		return clause.Expr{SQL: "NULL"}
	}
	
	return clause.Expr{
		SQL:  "encrypt_message(?, ?)",
		Vars: []interface{}{string(et), encryptionKey},
	}
}

func (m *Message) AfterFind(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		return nil
	}

	encryptionKey := platform.Get("MESSAGE_ENCRYPTION_KEY", "")
	if encryptionKey == "" {
		return fmt.Errorf("MESSAGE_ENCRYPTION_KEY not set")
	}

	var decryptedText string
	err := tx.Raw("SELECT decrypt_message(message_text, ?) FROM xi_messages WHERE id = ?", 
		encryptionKey, m.ID).Scan(&decryptedText).Error
	
	if err != nil {
		return err
	}

	m.MessageText = EncryptedText(decryptedText)
	return nil
}