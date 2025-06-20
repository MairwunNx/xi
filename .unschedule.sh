#!/bin/bash

set -e

echo "🛑 Остановка и удаление автоматического деплоя Xi Manager..."

if systemctl list-unit-files | grep -q "ximanager-autodeploy.timer"; then
    echo "⏸️ Останавливаем timer..."
    sudo systemctl stop ximanager-autodeploy.timer 2>/dev/null || true
    
    echo "🚫 Отключаем timer..."
    sudo systemctl disable ximanager-autodeploy.timer 2>/dev/null || true
    
    echo "🗑️ Удаляем файлы systemd..."
    sudo rm -f /etc/systemd/system/ximanager-autodeploy.service
    sudo rm -f /etc/systemd/system/ximanager-autodeploy.timer
    
    echo "🔄 Перезагружаем systemd daemon..."
    sudo systemctl daemon-reload
    
    echo "✅ Автоматический деплой успешно удален!"
else
    echo "ℹ️ Timer ximanager-autodeploy не найден"
fi

echo ""
echo "📊 Проверяем статус:"
if systemctl list-unit-files | grep -q "ximanager-autodeploy"; then
    echo "❌ Что-то пошло не так, timer все еще существует"
else
    echo "✅ Timer полностью удален"
fi

echo ""
echo "🧹 Для полной очистки также можете удалить логи:"
echo "   sudo journalctl --vacuum-time=1d"
echo "   sudo rm -f /var/log/ximanager-autodeploy.log"
echo "   sudo rm -f /var/log/ximanager-cron.log" 