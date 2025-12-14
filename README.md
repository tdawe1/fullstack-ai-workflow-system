# Kyros Praxis MVP

Multi-agent orchestration platform with CrewAI integration.

## Architecture

```
apps/
├── api/          # Python FastAPI backend (port 8001)
├── gateway/      # Go API gateway (port 8003)
└── console/      # React frontend
```

## Quick Start

### Backend (Python)

```bash
cd apps/api
python -m venv .venv && source .venv/bin/activate
pip install -e .
docker compose up -d db  # Start PostgreSQL
alembic upgrade head     # Run migrations
uvicorn app.main:app --port 8001
```

### Gateway (Go)

```bash
cd apps/gateway
go build -o bin/server ./cmd/server
./bin/server
```

### Frontend

```bash
cd apps/console
npm install
npm run dev
```

## Features

- **Multi-Agent Workflow**: Planner → Coder → Tester pipeline
- **Multi-Cloud LLM**: OpenRouter, OpenAI, Vertex AI, Bedrock, Azure
- **Real-time Updates**: SSE event streaming
- **WebSocket Terminal**: Interactive PTY access
- **JWT Authentication**: Secure token-based auth

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `POST /auth/register` | User registration |
| `POST /auth/login` | User login |
| `GET /projects` | List projects |
| `POST /projects` | Create project |
| `POST /projects/:id/generate` | Start AI workflow |
| `GET /admin/providers` | LLM provider status |

## Environment Variables

```bash
# Database
DATABASE_URL=postgres://kyros:kyros@localhost:5432/kyros

# JWT
JWT_SECRET_KEY=your-secret-key

# LLM Provider
MODEL_PROVIDER=openrouter
OPENROUTER_API_KEY=your-key

# Optional: Multi-cloud
GOOGLE_PROJECT_ID=your-gcp-project
AWS_REGION=us-east-1
AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
```

## License

MIT
