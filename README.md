# 🀄️ Xi Manager

[![AI Capable](https://img.shields.io/badge/AI-Capable-brightgreen?style=flat&logo=openai&logoColor=white)](https://github.com/mairwunnx/ximanager)

**Язык**: 🇷🇺 Русский | [🇺🇸 English](README.EN.md) | [🇨🇳 中文](README.CN.md)

**Xi Manager** — 🀄️ Telegram-бот с ИИ, стилизованный под личного помощника Xi. Личный помощник великого лидера, готовый отвечать на вопросы простого народа.

> **Внимание**: Это развлекательный проект, не имеющий отношения к реальным политическим деятелям.

## Фичи

### Основной функционал

- **Мультимодальность** — обработка текста, голосовых сообщений и изображений с помощью AI.
- **Несколько AI-провайдеров** — поддержка OpenAI, DeepSeek, Anthropic, xAI и OpenRouter.
- **Настраиваемые режимы** — различные стили общения и поведения бота.
- **Контекстная память** — сохраняет историю диалога между сессиями.
- **Статистика** — ведение статистики использования бота.
- **Закрепленные сообщения** — закрепление важных сообщений.
- **Менеджмент** — управление ботом и его функционалом.

### Система управления

- **Гибкая система прав** — управление доступом к функциям бота.
- **Управление пользователями** — администрирование доступа и прав.
- **Управление контекстом** — очистка и настройка памяти диалогов.
- **Режимы поведения** — создание и редактирование персонализированных режимов.

## Использование

### Начало работы
1. Найдите бота в Telegram: `@ximanager_bot`
2. Начните диалог командой `/start`

### Доступные команды

#### В личных сообщениях
- Просто пишите вопросы боту — он ответит с учётом контекста диалога

#### В групповых чатах
- `/xi <сообщение>` — обратиться к боту
- `/this` — информация о текущем чате
- `/stats` — статистика использования бота

### Как это работает

1. Отправьте сообщение боту (текст, голос или изображение)
2. Бот обработает запрос через выбранный AI-провайдер
3. Получите ответ с учётом контекста предыдущих сообщений
4. Контекст сохраняется между сессиями для непрерывного диалога

## Совместимость

- **Telegram Bot API** — работает в личных и групповых чатах
- **База данных**: PostgreSQL 13+
- **Кэш**: Redis 6.0+
- **Рантайм**: Go 1.25+
- **Деплоймент**: Docker + Docker Compose

## Деплоймент

### Dev Containers (рекомендуется для разработки)

Самый простой способ запуска для разработки — использование Dev Containers в VS Code или аналогичных IDE:

1. Клонируйте репозиторий:
```bash
git clone https://github.com/mairwunnx/xi
cd xi
```

2. Настройте обязательные переменные окружения (в файле `.env.dev`):
```bash
# Telegram Bot
TELEGRAM_BOT_TOKEN="ваш_telegram_bot_token"

# AI API Keys
OPENROUTER_API_KEY="ваш_openrouter_api_key"
OPENAI_API_KEY="ваш_openai_api_key"

# Agent Prompts (base64 encoded)
AGENT_CONTEXT_SELECTION_PROMPT="base64_encoded_prompt"
AGENT_MODEL_SELECTION_PROMPT="base64_encoded_prompt"
```

> **Примечание**: Готовые base64 промпты для агентов можно найти в файлах `prompt0.json` и `prompt1.json`.

> Так же можете сразу изменить промпт по умолчанию в `migrations/V4__create_modes_tables.sql`!

3. Откройте проект в VS Code и выберите "Reopen in Container"

Всё остальное (PostgreSQL, Redis, Flyway миграции, зависимости) настроится автоматически. Просто и быстро!

### Docker Compose (для продакшена)

1. Клонируйте репозиторий:
```bash
git clone https://github.com/mairwunnx/xi
cd xi
```

2. Настройте обязательные переменные окружения (в файле `.env`):
```bash
# Telegram Bot
TELEGRAM_BOT_TOKEN="ваш_telegram_bot_token"

# AI API Keys
OPENROUTER_API_KEY="ваш_openrouter_api_key"
OPENAI_API_KEY="ваш_openai_api_key"

# Agent Prompts (base64 encoded)
AGENT_CONTEXT_SELECTION_PROMPT="base64_encoded_prompt"
AGENT_MODEL_SELECTION_PROMPT="base64_encoded_prompt"
```

> **Примечание**: При необходимости настройте остальные переменные окружения (пароли для Redis и PostgreSQL, конфигурацию Prometheus/Grafana).

3. Запустите сервисы:
```bash
docker compose up -d
```

4. Проверьте статус:
```bash
docker compose logs -f ximanager
```

### Docker Compose с готовым образом

Если хотите использовать готовый образ и управлять PostgreSQL/Redis самостоятельно:

1. Задекларируйте сервис ximanager в `docker-compose.yml`:

```yaml
services:
  ximanager:
    image: ghcr.io/mairwunnx/ximanager:4.2.6
    env_file: .env
    environment:
      REDIS_ADDRESS: ${REDIS_ADDRESS}
      REDIS_PASSWORD: ${REDIS_PASSWORD}
      POSTGRES_HOST: ${POSTGRES_HOST}
      POSTGRES_PORT: ${POSTGRES_PORT}
      POSTGRES_DATABASE: ${POSTGRES_DATABASE}
      POSTGRES_USERNAME: ${POSTGRES_USERNAME}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      TELEGRAM_BOT_TOKEN: ${TELEGRAM_BOT_TOKEN}
      OPENROUTER_API_KEY: ${OPENROUTER_API_KEY}
      OPENAI_API_KEY: ${OPENAI_API_KEY}
      AGENT_CONTEXT_SELECTION_PROMPT: ${AGENT_CONTEXT_SELECTION_PROMPT}
      AGENT_MODEL_SELECTION_PROMPT: ${AGENT_MODEL_SELECTION_PROMPT}
```

> **Примечание**: Вам потребуется передать все переменные окружения из файла `.env` или любым другим удобным способом.

2. Задекларируйте сервисы PostgreSQL и Redis в `docker-compose.yml`. 
> Важно, версия **PostgreSQL** должна быть `13+`, а **Redis** должна быть `6.0+`.

3. Настройте миграции базы данных с помощью Flyway или примените их вручную из папки `migrations/`.

4. Запустите сервисы:
```bash
docker compose up -d
```

### Сборка из исходников

#### Ручной и прямой способ сборки

Требования:
- Go 1.25+

```bash
go build -o ximanager program.go
```

#### Сборка с помощью Docker

Требования:
- Docker

```bash
docker build -t ximanager .
```

## Стек

- **Go 1.25** — основной язык разработки
- **Telegram Bot API (tgbotapi)** — интеграция с Telegram
- **PostgreSQL + GORM** — основная база данных с ORM и автогенерацией DAO
- **Redis + go-redis** — кэширование сессий и быстрые операции
- **Flyway** — управление миграциями базы данных
- **OpenAI API** — GPT модели
- **DeepSeek API** — DeepSeek модели
- **Anthropic API** — Claude модели
- **xAI API** — Grok модели
- **OpenRouter** — агрегатор AI-моделей для расширения возможностей
- **Whisper** — распознавание голосовых сообщений
- **Uber-FX** — dependency injection для модульной архитектуры
- **Structured Logging (slog)** — JSON логирование с контекстом
- **Docker + Docker Compose** — контейнеризация и оркестрация
- **Prometheus + Grafana** — мониторинг и метрики

## Участие AI

AI использовался для оптимизации промптов, улучшения архитектуры модулей, а также для создания части документации и генерации GORM моделей, а так же для названий коммитов.

Так же на момент 22 сентября 2025 года, добавлены AI хитнты (промпты/рулы) в папку `.cursor/rules/`, которые могут помочь при внедрении новых фич, с использованием LLM агентов в будущем.

## Ссылки на связанные проекты

[Dickobrazz](https://github.com/mairwunnx/dickobrazz) — 🌶️ Дикобраз бот, он же дикобот, способен в точности до сантиметра выдать размер вашего агрегата. Современный и технологичный кокомер с системой сезонов и геймификацией.

[Louisepizdon](https://github.com/MairwunNx/louisepizdon) — 🥀 Луипиздон, Telegram-бот с ИИ, который честнее чем твоя бабушка. Оценит тебя по достоинству, разборка ценообразования твоих шмоток с фотографии!

## Из серии "от того же автора"

[Mo'Bosses](https://github.com/mairwunnx/mobosses) — 🏆 **Mo'Bosses** — это лучший RPG плагин, который превращает обычных мобов в эпических боссов с **продвинутой системой прогрессии игрока**. В отличие от других плагинов, здесь каждый бой имеет значение, а каждый уровень открывает новые возможности! ⚔

[Mo'Joins](https://github.com/mairwunnx/mojoins) — 🎉 Кастомные входы/выходы: сообщения, звуки, частицы, фейерверки и защита после входа. Все для PaperMC.

[Mo'Afks](https://github.com/mairwunnx/moafks) — 🛡️ Пауза в онлайне — теперь возможна. Плагин для PaperMC, который даёт игроку безопасный режим AFK: иммунитет к урону, отсутствие коллизий, игнор мобами, авто-детект неактивности и аккуратные визуальные эффекты.

[McBuddy Server](https://github.com/mcbuddy-ai/mcbuddy-server) — 🛠️⚡ Бэкенд для AI-ассистента MCBuddy с интеграцией OpenRouter и обработкой запросов

[McBuddy Telegram](https://github.com/mcbuddy-ai/mcbuddy-bot) — 🤖📱 Telegram-бот для общения с MCBuddy за пределами игры

[McBuddy Spigot](https://github.com/mcbuddy-ai/mcbuddy-spigot) — 💬 Spigot-плагин для интеграции MCBuddy — добавляет команду `/ask` для вопросов к AI-ассистенту прямо в чате Minecraft сервера! 🎮

---

![image](./media.jpg)

🇷🇺 **Сделано в России с любовью.** ❤️

**Xi Manager** — это про качественный китайский AI-ассистент и современные технологии. За народ!

> 🫡 Made by Pavel Erokhin (Павел Ерохин), aka mairwunnx.