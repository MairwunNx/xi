#!/bin/bash

set -e

echo "🚀 Начинаем деплой Xi Manager..."

echo "🏗️ Собираем образ XiManager..."
docker compose build ximanager

echo "🛑 Останавливаем текущие контейнеры..."
docker compose down

echo "🏗️ Собираем и запускаем контейнеры..."
docker compose up -d

echo "✅ Деплой завершен!"
echo ""
echo "📝 Статус: docker compose ps"
echo "📝 Логи XiManager: docker compose logs -f ximanager"
echo "📝 Логи Postgres: docker compose logs -f postgres"
echo "📝 Логи Flyway: docker compose logs -f flyway"
echo ""
echo "🔑 Интерактив с XiManager: docker exec -it ximanager bash"
echo ""
echo "🚀 Загрузка XiManager: docker stats ximanager"