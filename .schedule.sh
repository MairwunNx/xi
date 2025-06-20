#!/bin/bash

set -e

echo "⏰ Настройка автоматического деплоя Xi Manager через Systemd Timer..."

CURRENT_DIR=$(pwd)
CURRENT_USER=$(whoami)
SCRIPT_PATH="$CURRENT_DIR/.deploy.sh"

USER_HOME=$(eval echo ~$CURRENT_USER)
XIMANAGER_DIR=$(find "$USER_HOME" -maxdepth 2 -type d -name "ximanager*" | head -1)

if [ -z "$XIMANAGER_DIR" ]; then
    echo "❌ Не найдена директория ximanager* в $USER_HOME"
    echo "💡 Используем текущую директорию: $CURRENT_DIR"
    XIMANAGER_DIR="$CURRENT_DIR"
fi

echo "📁 Рабочая директория: $XIMANAGER_DIR"
echo "👤 Пользователь: $CURRENT_USER"

echo "🔧 Делаем .deploy.sh исполняемым..."
chmod +x .deploy.sh

echo "📝 Создаем systemd service файл..."

sudo tee /etc/systemd/system/ximanager-autodeploy.service > /dev/null <<EOF
[Unit]
Description=Xi Manager Auto Deploy Service
After=network.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=oneshot
ExecStart=$SCRIPT_PATH
User=$CURRENT_USER
Group=$CURRENT_USER
WorkingDirectory=$XIMANAGER_DIR
Environment=HOME=$USER_HOME
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=DOCKER_HOST=unix:///var/run/docker.sock

# Логирование
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ximanager-autodeploy

# Права для работы с Docker через sudo
SupplementaryGroups=docker
# Разрешаем sudo для docker команд
NoNewPrivileges=false

# Таймауты
TimeoutStartSec=300
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
EOF

echo "⏲️ Создаем systemd timer файл (проверка каждую минуту)..."

sudo tee /etc/systemd/system/ximanager-autodeploy.timer > /dev/null <<EOF
[Unit]
Description=Xi Manager Auto Deploy Timer - каждую минуту
Requires=ximanager-autodeploy.service

[Timer]
# Запуск каждую минуту
OnCalendar=*:*:00
# Запуск при старте системы (через 2 минуты)
OnBootSec=2min
# Сохранять время последнего запуска
Persistent=true
# Точность (по умолчанию 1 минута)
AccuracySec=1s

[Install]
WantedBy=timers.target
EOF

echo "🔄 Перезагружаем systemd daemon..."
sudo systemctl daemon-reload

echo "✅ Включаем и запускаем timer..."
sudo systemctl enable ximanager-autodeploy.timer
sudo systemctl start ximanager-autodeploy.timer

echo ""
echo "🎉 Systemd Timer успешно настроен!"
echo ""
echo "📊 Информация о настройке:"
echo "   🕐 Интервал проверки: каждую минуту"
echo "   📁 Рабочая директория: $XIMANAGER_DIR"
echo "   👤 Пользователь: $CURRENT_USER"
echo "   🐳 Docker: поддержка sudo включена"
echo "   📝 Service: ximanager-autodeploy.service"
echo "   ⏲️ Timer: ximanager-autodeploy.timer"
echo ""
echo "🔧 Полезные команды:"
echo "   📊 Статус timer: sudo systemctl status ximanager-autodeploy.timer"
echo "   📊 Статус service: sudo systemctl status ximanager-autodeploy.service"
echo "   📝 Логи: sudo journalctl -u ximanager-autodeploy.service -f"
echo "   📝 Логи timer: sudo journalctl -u ximanager-autodeploy.timer -f"
echo "   ⏸️ Остановка: sudo systemctl stop ximanager-autodeploy.timer"
echo "   ▶️ Запуск: sudo systemctl start ximanager-autodeploy.timer"
echo "   🚫 Отключение: sudo systemctl disable ximanager-autodeploy.timer"
echo "   🔄 Перезапуск: sudo systemctl restart ximanager-autodeploy.timer"
echo ""
echo "🕐 Следующие запуски timer:"
sudo systemctl list-timers ximanager-autodeploy.timer
echo ""
echo "📋 Для проверки работы можете запустить вручную:"
echo "   sudo systemctl start ximanager-autodeploy.service"
echo ""
echo "✨ Автоматический деплой настроен и запущен!" 