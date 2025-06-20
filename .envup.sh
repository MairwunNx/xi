#!/bin/bash

set -e

echo "🚀 Начинаем деплой Xi Manager..."

echo "🏗️ Собираем образ XiManager..."
sudo docker compose build ximanager

echo "🛑 Останавливаем текущие контейнеры..."
sudo docker compose down

echo "🏗️ Собираем и запускаем контейнеры..."
sudo docker compose up -d

echo "✅ Деплой завершен!"
echo ""
echo "📝 Статус: sudo docker compose ps"
echo "📝 Логи XiManager: sudo docker compose logs -f ximanager"
echo "📝 Логи Postgres: sudo docker compose logs -f postgres"
echo "📝 Логи Flyway: sudo docker compose logs -f flyway"
echo ""
echo "🔑 Интерактив с XiManager: sudo docker exec -it ximanager bash"
echo ""
echo "🚀 Загрузка XiManager: sudo docker stats ximanager"