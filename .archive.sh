#!/bin/bash

echo "🚀 Начинаем архивацию Xi Manager..."

git archive --format=zip --add-file .env --output ./ximanager.zip master
zip -ur ./ximanager.zip .git/

echo "✅ Архивация завершена!"
