# 🀄️ Xi Manager

[![AI Capable](https://img.shields.io/badge/AI-Capable-brightgreen?style=flat&logo=openai&logoColor=white)](https://github.com/mairwunnx/ximanager)

**语言**: [🇷🇺 Русский](README.md) | [🇺🇸 English](README.EN.md) | 🇨🇳 中文

**Xi Manager** — 🀄️ 基于 AI 的 Telegram 机器人，风格化为 Xi 的私人助理。伟大领袖的私人助理，随时准备回答人民群众的问题。

> **注意**：这是一个娱乐项目，与真实政治人物无关。

## 功能特性

### 核心功能

- **多模态** — 使用 AI 处理文本、语音消息和图像。
- **多个 AI 提供商** — 支持 OpenAI、DeepSeek、Anthropic、xAI 和 OpenRouter。
- **可自定义模式** — 各种通信风格和机器人行为。
- **上下文记忆** — 在会话之间保留对话历史。
- **统计数据** — 跟踪机器人使用统计。
- **固定消息** — 固定重要消息。
- **管理功能** — 机器人和功能管理。

### 管理系统

- **灵活的权限系统** — 对机器人功能的访问控制。
- **用户管理** — 访问和权限管理。
- **上下文管理** — 清除和配置对话记忆。
- **行为模式** — 创建和编辑个性化模式。

## 使用方法

### 开始使用
1. 在 Telegram 上找到机器人：`@ximanager_bot`
2. 使用 `/start` 命令开始对话

### 可用命令

#### 在私聊中
- 直接向机器人提问 — 它将考虑对话上下文进行回应

#### 在群聊中
- `/xi <消息>` — 向机器人发送消息
- `/this` — 当前聊天的信息
- `/stats` — 机器人使用统计

### 工作原理

1. 向机器人发送消息（文本、语音或图像）
2. 机器人通过选定的 AI 提供商处理请求
3. 接收考虑到先前消息上下文的响应
4. 上下文在会话之间保留，实现连续对话

## 兼容性

- **Telegram Bot API** — 在私聊和群聊中工作
- **数据库**: PostgreSQL 13+
- **缓存**: Redis 6.0+
- **运行时**: Go 1.25+
- **部署**: Docker + Docker Compose

## 部署

### Dev Containers（推荐用于开发）

开发环境最简单的启动方式是在 VS Code 或类似的 IDE 中使用 Dev Containers：

1. 克隆仓库：
```bash
git clone https://github.com/mairwunnx/xi
cd xi
```

2. 配置必需的环境变量（在 `.env.dev` 文件中）：
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

> **注意**：预制的 base64 代理提示可以在 `prompt0.json` 和 `prompt1.json` 文件中找到。

> 您还可以立即更改 `migrations/V4__create_modes_tables.sql` 中的默认提示！

3. 在 VS Code 中打开项目并选择 "Reopen in Container"

其他所有内容（PostgreSQL、Redis、Flyway 迁移、依赖项）将自动配置。简单快捷！

### Docker Compose（用于生产环境）

1. 克隆仓库：
```bash
git clone https://github.com/mairwunnx/xi
cd xi
```

2. 配置必需的环境变量（在 `.env` 文件中）：
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

> **注意**：根据需要配置其他环境变量（Redis 和 PostgreSQL 的密码，Prometheus/Grafana 配置）。

3. 启动服务：
```bash
docker compose up -d
```

4. 检查状态：
```bash
docker compose logs -f ximanager
```

### 使用预构建镜像的 Docker Compose

如果您想使用预构建镜像并自行管理 PostgreSQL/Redis：

1. 在 `docker-compose.yml` 中声明 ximanager 服务：

```yaml
services:
  ximanager:
    image: ghcr.io/mairwunnx/ximanager:4.2.5
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

> **注意**：您需要从 `.env` 文件或以任何其他方便的方式传递所有环境变量。

2. 在 `docker-compose.yml` 中声明 PostgreSQL 和 Redis 服务。
> 重要：**PostgreSQL** 版本应为 `13+`，**Redis** 应为 `6.0+`。

3. 使用 Flyway 配置数据库迁移或从 `migrations/` 文件夹手动应用。

4. 启动服务：
```bash
docker compose up -d
```

### 从源代码构建

#### 手动直接构建

要求：
- Go 1.25+

```bash
go build -o ximanager program.go
```

#### 使用 Docker 构建

要求：
- Docker

```bash
docker build -t ximanager .
```

## 技术栈

- **Go 1.25** — 主要开发语言
- **Telegram Bot API (tgbotapi)** — Telegram 集成
- **PostgreSQL + GORM** — 主数据库，带 ORM 和 DAO 自动生成
- **Redis + go-redis** — 会话缓存和快速操作
- **Flyway** — 数据库迁移管理
- **OpenAI API** — GPT 模型
- **DeepSeek API** — DeepSeek 模型
- **Anthropic API** — Claude 模型
- **xAI API** — Grok 模型
- **OpenRouter** — AI 模型聚合器，扩展功能
- **Whisper** — 语音消息识别
- **Uber-FX** — 模块化架构的依赖注入
- **Structured Logging (slog)** — 带上下文的 JSON 日志记录
- **Docker + Docker Compose** — 容器化和编排
- **Prometheus + Grafana** — 监控和指标

## AI 参与

AI 用于提示优化、模块架构改进，以及创建部分文档和生成 GORM 模型，还用于提交名称。

此外，截至 2025 年 9 月 22 日，AI 提示（prompts/rules）已添加到 `.cursor/rules/` 文件夹中，可以在未来使用 LLM 代理实现新功能时提供帮助。

## 相关项目链接

[Dickobrazz](https://github.com/mairwunnx/dickobrazz) — 🌶️ Dickobrazz 机器人，又名 dicobot，能够精确到厘米测量你的单位大小。现代化的技术型测量器，带有赛季系统和游戏化。

[Louisepizdon](https://github.com/MairwunNx/louisepizdon) — 🥀 Louisepizdon，一个比你奶奶还诚实的 AI Telegram 机器人。会正确评估你，根据照片分析你衣服的定价！

## 来自"同一作者"系列

[Mo'Bosses](https://github.com/mairwunnx/mobosses) — 🏆 **Mo'Bosses** 是最好的 RPG 插件，将普通的怪物转变为史诗级的 Boss，拥有**高级玩家进阶系统**。与其他插件不同，这里每场战斗都很重要，每个等级都会开启新的可能性！⚔

[Mo'Joins](https://github.com/mairwunnx/mojoins) — 🎉 自定义加入/退出：消息、声音、粒子、烟花和加入后的保护。全部用于 PaperMC。

[Mo'Afks](https://github.com/mairwunnx/moafks) — 🛡️ 在线时间暂停 — 现在可能了。PaperMC 插件，为玩家提供安全的 AFK 模式：伤害免疫、无碰撞、被怪物忽略、自动检测不活动和整洁的视觉效果。

[McBuddy Server](https://github.com/mcbuddy-ai/mcbuddy-server) — 🛠️⚡ McBuddy AI 助手的后端，集成 OpenRouter 和请求处理

[McBuddy Telegram](https://github.com/mcbuddy-ai/mcbuddy-bot) — 🤖📱 用于在游戏外与 McBuddy 通信的 Telegram 机器人

[McBuddy Spigot](https://github.com/mcbuddy-ai/mcbuddy-spigot) — 💬 McBuddy 集成的 Spigot 插件 — 添加 `/ask` 命令，直接在 Minecraft 服务器聊天中向 AI 助手提问！🎮

---

![image](./media.jpg)

🇷🇺 **在俄罗斯用爱制作。** ❤️

**Xi Manager** — 关于优质的中国 AI 助手和现代技术。为人民服务！

> 🫡 Made by Pavel Erokhin (Павел Ерохин), aka mairwunnx.

