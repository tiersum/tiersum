# Configuration Reference

Key configuration file: `configs/config.yaml` (copy from `configs/config.example.yaml`).

## Required Settings

```yaml
llm:
  provider: openai  # Options: openai, anthropic, local (ollama)
  openai:
    api_key: ${OPENAI_API_KEY}  # Required for openai provider
```

## LLM Provider Options

### OpenAI (Default)

```yaml
llm:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
    max_tokens: 2000
    temperature: 0.3
```

### Anthropic Claude

```yaml
llm:
  provider: anthropic
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    base_url: https://api.anthropic.com
    model: claude-3-haiku-20240307
    max_tokens: 2000
    temperature: 0.3
```

### Local / Ollama

```yaml
llm:
  provider: local
  local:
    base_url: http://localhost:11434
    model: llama3.2
    timeout: 60s
```

### OpenAI-Compatible Providers

Any provider with OpenAI-compatible API can use `provider: openai` with custom `base_url`:

| Provider        | base_url                                                              | Example Model                |
| --------------- | --------------------------------------------------------------------- | ---------------------------- |
| DeepSeek        | `https://api.deepseek.com/v1`                                         | `deepseek-chat`              |
| Groq            | `https://api.groq.com/openai/v1`                                      | `llama-3.1-8b-instant`       |
| Zhipu AI (GLM)  | `https://open.bigmodel.cn/api/paas/v4`                                | `glm-4-flash`                |
| Moonshot (Kimi) | `https://api.moonshot.cn/v1`                                          | `moonshot-v1-8k`             |
| OpenRouter      | `https://openrouter.ai/api/v1`                                        | `anthropic/claude-3.5-haiku` |
| SiliconFlow     | `https://api.siliconflow.cn/v1`                                       | `deepseek-ai/DeepSeek-V2.5`  |
| Azure OpenAI    | `https://{resource}.openai.azure.com/openai/deployments/{deployment}` | deployment name              |

Example for DeepSeek:

```yaml
llm:
  provider: openai
  openai:
    api_key: ${DEEPSEEK_API_KEY}
    base_url: https://api.deepseek.com/v1
    model: deepseek-chat
    max_tokens: 2000
    temperature: 0.3
```

## Database

### SQLite (Default)

```yaml
storage:
  database:
    driver: sqlite3
    dsn: ./data/tiersum.db
```

### PostgreSQL

```yaml
storage:
  database:
    driver: postgres
    dsn: postgres://user:password@localhost:5432/tiersum?sslmode=disable
```

## Key Configuration Sections

| Section             | Purpose                                        |
| ------------------- | ---------------------------------------------- |
| `server`            | HTTP port, CORS, timeouts                      |
| `llm`               | LLM provider settings                          |
| `storage.database`  | SQLite (default) or PostgreSQL                 |
| `quota`             | Hot document rate limiting (default: 100/hour) |
| `cold_index`        | Cold index: markdown split, hybrid search, embeddings |
| `documents.tiering` | Hot/cold thresholds                            |
| `mcp`               | MCP protocol settings                          |
| `auth.browser`      | Session TTL, max devices, CSRF, cookie security |

For all available keys, see `configs/config.example.yaml`.
