# 🀄️ Xi Manager

[![AI Capable](https://img.shields.io/badge/AI-Capable-brightgreen?style=flat&logo=openai&logoColor=white)](https://github.com/mairwunnx/ximanager)

**Language**: [🇷🇺 Русский](README.md) | 🇺🇸 English

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
- **Runtime**: Go 1.23+
- **Deployment**: Docker + Docker Compose

## Installation

### Fastest Launch with Docker Compose

1. You need to declare Xi Manager as a service using the ready-made image:

```yaml
services:
  ximanager:
    image: ghcr.io/mairwunnx/ximanager:latest
```

2. Start the service after feeding it environment variables.

### Quick Start with Docker Compose

1. Clone the repository:
```bash
git clone https://github.com/mairwunnx/xi
cd xi
```

2. Configure environment variables:
```bash
# Copy the configuration example
cp .env.draft .env

# Edit the .env file, filling in the necessary variables
```

> You can also immediately change the default prompt in `V4__create_modes_tables.sql`! (or change the default prompt later in the database)

3. Start the services:
```bash
docker compose up -d
```

4. Check the status:
```bash
docker compose logs -f ximanager
```

### Building from Source

Requirements:
- Go 1.23+
- Python 3 (for interop)
- Docker for containerization

```bash
# Generate GORM DAO
go run sources/persistence/gormgen/zygote.go

# Build Docker image
docker build -t xi-manager .
```

## Tech Stack

- **Go 1.23** — main development language with modular architecture
- **Telegram Bot API (tgbotapi)** — Telegram integration
- **PostgreSQL + GORM** — main database with ORM and auto-generation
- **Redis + go-redis** — session caching and fast operations
- **OpenAI, DeepSeek, Anthropic, xAI APIs** — multiple AI providers
- **OpenRouter** — AI model aggregator for expanded capabilities
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
