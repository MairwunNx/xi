# 🀄️ Xi Manager

[![AI Capable](https://img.shields.io/badge/AI-Capable-brightgreen?style=flat&logo=openai&logoColor=white)](https://github.com/mairwunnx/xi)
[![Docker](https://img.shields.io/badge/Docker-Available-2496ED?style=flat&logo=docker&logoColor=white)](https://github.com/MairwunNx/xi/pkgs/container/ximanager)
[![GitHub Release](https://img.shields.io/github/v/release/mairwunnx/xi?style=flat&logo=github&color=blue)](https://github.com/mairwunnx/xi/releases)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev/)

**Language**: [🇷🇺 Русский](README.md) | 🇺🇸 English | [🇨🇳 中文](README.CN.md)

**Xi Manager** — 🀄️ AI-powered Telegram bot styled as Xi's personal assistant. A personal assistant to the great leader, ready to answer questions from the common people.

> **Attention**: This is an entertainment project that has no relation to real political figures.

## Features

### Core Functionality

- **Multimodal** — processing text, voice messages, and images using AI.
- **Multiple AI providers** — support for OpenAI, DeepSeek, Anthropic, xAI, and OpenRouter.
- **Customizable modes** — various communication styles and bot behaviors.
- **Contextual memory** — preserves dialogue history between sessions.
- **Statistics** — tracking bot usage statistics.
- **Pinned messages** — pinning important messages.
- **Management** — bot and functionality management.

### Management System

- **Flexible permissions system** — access control to bot functions.
- **User management** — access and permissions administration.
- **Context management** — clearing and configuring dialogue memory.
- **Behavior modes** — creating and editing personalized modes.

## Usage

### Getting Started
1. Find the bot on Telegram: `@ximanager_bot`
2. Start a conversation with the `/start` command

### Available Commands

#### In private messages
- Simply write questions to the bot — it will respond considering the dialogue context

#### In group chats
- `/xi <message>` — address the bot
- `/this` — information about the current chat
- `/stats` — bot usage statistics

### How It Works

1. Send a message to the bot (text, voice, or image)
2. The bot processes the request through the selected AI provider
3. Receive a response considering the context of previous messages
4. Context is preserved between sessions for continuous dialogue

## Compatibility

- **Telegram Bot API** — works in private and group chats
- **Database**: PostgreSQL 13+
- **Cache**: Redis 6.0+
- **Runtime**: Go 1.25+
- **Deployment**: Docker + Docker Compose

## Deployment

### Dev Containers (recommended for development)

The easiest way to launch for development is to use Dev Containers in VS Code or similar IDEs:

1. Clone the repository:
```bash
git clone https://github.com/mairwunnx/xi
cd xi
```

2. Configure required environment variables (in `.env.dev` file):
```bash
# Telegram Bot
TELEGRAM_BOT_TOKEN="your_telegram_bot_token"

# AI API Keys
OPENROUTER_API_KEY="your_openrouter_api_key"
OPENAI_API_KEY="your_openai_api_key"

# Agent Prompts (base64 encoded)
AGENT_CONTEXT_SELECTION_PROMPT="base64_encoded_prompt"
AGENT_MODEL_SELECTION_PROMPT="base64_encoded_prompt"
```

> **Note**: Pre-made base64 agent prompts can be found in `prompt0.json` and `prompt1.json` files.

> You can also immediately change the default prompt in `migrations/V4__create_modes_tables.sql`!

3. Open the project in VS Code and select "Reopen in Container"

Everything else (PostgreSQL, Redis, Flyway migrations, dependencies) will be configured automatically. Simple and fast!

### Docker Compose (for production)

1. Clone the repository:
```bash
git clone https://github.com/mairwunnx/xi
cd xi
```

2. Configure required environment variables (in `.env` file):
```bash
# Telegram Bot
TELEGRAM_BOT_TOKEN="your_telegram_bot_token"

# AI API Keys
OPENROUTER_API_KEY="your_openrouter_api_key"
OPENAI_API_KEY="your_openai_api_key"

# Agent Prompts (base64 encoded)
AGENT_CONTEXT_SELECTION_PROMPT="base64_encoded_prompt"
AGENT_MODEL_SELECTION_PROMPT="base64_encoded_prompt"
```

> **Note**: Configure other environment variables as needed (passwords for Redis and PostgreSQL, Prometheus/Grafana configuration).

3. Start the services:
```bash
docker compose up -d
```

4. Check the status:
```bash
docker compose logs -f ximanager
```

### Docker Compose with Pre-built Image

If you want to use a pre-built image and manage PostgreSQL/Redis yourself:

1. Declare the ximanager service in `docker-compose.yml`:

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

> **Note**: You will need to pass all environment variables from the `.env` file or any other convenient way.

2. Declare PostgreSQL and Redis services in `docker-compose.yml`. 
> Important: **PostgreSQL** version should be `13+`, and **Redis** should be `6.0+`.

3. Configure database migrations using Flyway or apply them manually from the `migrations/` folder.

4. Start the services:
```bash
docker compose up -d
```

### Building from Source

#### Manual and Direct Build

Requirements:
- Go 1.25+

```bash
go build -o ximanager program.go
```

#### Building with Docker

Requirements:
- Docker

```bash
docker build -t ximanager .
```

## Tech Stack

- **Go 1.25** — main development language
- **Telegram Bot API (tgbotapi)** — Telegram integration
- **PostgreSQL + GORM** — main database with ORM and DAO auto-generation
- **Redis + go-redis** — session caching and fast operations
- **Flyway** — database migration management
- **OpenAI API** — GPT models
- **DeepSeek API** — DeepSeek models
- **Anthropic API** — Claude models
- **xAI API** — Grok models
- **OpenRouter** — AI model aggregator for expanded capabilities
- **Whisper** — voice message recognition
- **Uber-FX** — dependency injection for modular architecture
- **Structured Logging (slog)** — JSON logging with context
- **Docker + Docker Compose** — containerization and orchestration
- **Prometheus + Grafana** — monitoring and metrics

## AI Participation

AI was used for prompt optimization, module architecture improvement, as well as for creating part of the documentation and generating GORM models, and also for commit names.

Also, as of September 22, 2025, AI hints (prompts/rules) have been added to the `.cursor/rules/` folder, which can help when implementing new features using LLM agents in the future.

## Links to Related Projects

[Dickobrazz](https://github.com/mairwunnx/dickobrazz) — 🌶️ Dickobrazz bot, aka dicobot, capable of measuring your unit size to the nearest centimeter. A modern and technological cockometer with seasons system and gamification.

[Louisepizdon](https://github.com/MairwunNx/louisepizdon) — 🥀 Louisepizdon, an AI Telegram bot that's more honest than your grandmother. Will evaluate you properly, breaking down the pricing of your clothes from a photo!

## From the Series "By the Same Author"

[Mo'Bosses](https://github.com/mairwunnx/mobosses) — 🏆 **Mo'Bosses** is the best RPG plugin that transforms ordinary mobs into epic bosses with an **advanced player progression system**. Unlike other plugins, here every fight matters, and each level opens new possibilities! ⚔

[Mo'Joins](https://github.com/mairwunnx/mojoins) — 🎉 Custom joins/quits: messages, sounds, particles, fireworks, and protection after joining. All for PaperMC.

[Mo'Afks](https://github.com/mairwunnx/moafks) — 🛡️ Pause in online time — now possible. A plugin for PaperMC that gives players a safe AFK mode: damage immunity, no collisions, ignored by mobs, auto-detect inactivity, and neat visual effects.

[McBuddy Server](https://github.com/mcbuddy-ai/mcbuddy-server) — 🛠️⚡ Backend for MCBuddy AI assistant with OpenRouter integration and request processing

[McBuddy Telegram](https://github.com/mcbuddy-ai/mcbuddy-bot) — 🤖📱 Telegram bot for communicating with MCBuddy outside the game

[McBuddy Spigot](https://github.com/mcbuddy-ai/mcbuddy-spigot) — 💬 Spigot plugin for MCBuddy integration — adds `/ask` command for AI assistant questions directly in Minecraft server chat! 🎮

---

![image](./media.jpg)

🇷🇺 **Made in Russia with love.** ❤️

**Xi Manager** — is about quality Chinese AI assistant and modern technologies. For the people!

> 🫡 Made by Pavel Erokhin (Павел Ерохин), aka mairwunnx.
