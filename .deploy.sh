#!/bin/bash

set -e

REPO_DIR=$(find . -maxdepth 2 -type d -name "xi" | head -1)

if [ -z "$REPO_DIR" ]; then
    REPO_DIR="${1:-$(pwd)}"
fi

BRANCH="master"
LOG_FILE="/var/log/ximanager-autodeploy.log"

if [ -f "$REPO_DIR/.env" ]; then
    echo "🔧 Загружаем переменные из .env файла..."
    export $(grep -v '^#' "$REPO_DIR/.env" | grep -v '^$' | xargs)
    echo "✅ Переменные окружения загружены"
else
    echo "ℹ️ Файл .env не найден, используем системные переменные"
fi

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" | sudo tee -a "$LOG_FILE"
}

log "🔄 Начинаем автоматический деплой Xi Manager..."

cd "$REPO_DIR" || {
    log "❌ Ошибка: не удалось перейти в директорию $REPO_DIR"
    exit 1
}

if ! git status &>/dev/null; then
    log "❌ Ошибка: директория не является git репозиторием"
    exit 1
fi

CURRENT_COMMIT=$(git rev-parse HEAD)
log "📍 Текущий коммит: $CURRENT_COMMIT"

log "⬇️ Пуллим изменения из репозитория..."
if git pull origin "$BRANCH"; then
    NEW_COMMIT=$(git rev-parse HEAD)
    log "📍 Новый коммит: $NEW_COMMIT"
    
    if [ "$CURRENT_COMMIT" != "$NEW_COMMIT" ]; then
        log "🆕 Обнаружены новые изменения, начинаем деплой..."
        
        MIGRATION_BACKUP=""
        if [ -n "$MAGIC_PROMPT" ]; then
            log "🔧 Подставляем magic prompt из переменной окружения..."
            
            MIGRATION_BACKUP="migrations/V4__create_modes_tables.sql.backup"
            cp migrations/V4__create_modes_tables.sql "$MIGRATION_BACKUP"
            
            python3 -c "
import os
with open('migrations/V4__create_modes_tables.sql', 'r') as f:
    content = f.read()
content = content.replace('{{magic_prompt}}', os.environ.get('MAGIC_PROMPT', '{{magic_prompt}}'))
with open('migrations/V4__create_modes_tables.sql', 'w') as f:
    f.write(content)
"
            
            log "✅ Magic prompt подставлен"
        else
            log "⚠️ Переменная MAGIC_PROMPT не найдена, используем placeholder"
        fi
        
        restore_backup() {
            if [ -n "$MIGRATION_BACKUP" ] && [ -f "$MIGRATION_BACKUP" ]; then
                log "🔄 Восстанавливаем оригинальную миграцию..."
                mv "$MIGRATION_BACKUP" migrations/V4__create_modes_tables.sql
                log "✅ Оригинальная миграция восстановлена"
            fi
        }
        
        if ./.envup.sh; then
            log "✅ Деплой успешно завершен!"
            
            restore_backup

            if [ -n "$DEPLOY_BOT_TOKEN" ] && [ -n "$DEPLOY_CHAT_ID" ]; then
                log "📱 Отправляем уведомление в Telegram..."
                TELEGRAM_MESSAGE="🚀 Xi Manager успешно обновлен!%0A📍 Коммит: $NEW_COMMIT%0A⏰ Время: $(date '+%Y-%m-%d %H:%M:%S')"
                
                if curl -s -X POST "https://api.telegram.org/bot$DEPLOY_BOT_TOKEN/sendMessage" \
                     -d "chat_id=$DEPLOY_CHAT_ID" \
                     -d "text=$TELEGRAM_MESSAGE" \
                     -d "parse_mode=HTML" > /dev/null; then
                    log "✅ Уведомление в Telegram отправлено"
                else
                    log "⚠️ Не удалось отправить уведомление в Telegram"
                fi
            else
                log "ℹ️ Переменные BOT_TOKEN или DEPLOY_CHAT_ID не настроены, уведомления отключены"
            fi
        else
            log "❌ Ошибка при деплое!"
            
            restore_backup

            if [ -n "$DEPLOY_BOT_TOKEN" ] && [ -n "$DEPLOY_CHAT_ID" ]; then
                ERROR_MESSAGE="❌ Ошибка деплоя Xi Manager!%0A📍 Коммит: $NEW_COMMIT%0A⏰ Время: $(date '+%Y-%m-%d %H:%M:%S')"
                curl -s -X POST "https://api.telegram.org/bot$DEPLOY_BOT_TOKEN/sendMessage" \
                     -d "chat_id=$DEPLOY_CHAT_ID" \
                     -d "text=$ERROR_MESSAGE" > /dev/null || true
            fi
            exit 1
        fi
    else
        log "ℹ️ Новых изменений не обнаружено"
    fi
else
    log "❌ Ошибка при пуллинге изменений"
    exit 1
fi

log "🏁 Автоматический деплой завершен" 