# Architectural Deep Clean - Codebase Audit Report

**Auditor**: Axiom (Principal Software Engineer & Security Auditor)  
**Date**: 2025-12-15  
**Application**: Fullstack AI Workflow System

---

# PHASE 1: Code Cartography & De-tangling

## High-Level Purpose

This is a **multi-agent AI orchestration platform for code generation and review**. The application allows users to:

1. Create projects and define tasks
2. Submit prompts for AI-powered code generation
3. Run a hierarchical multi-agent pipeline (Planner → Coder → Tester) that iteratively generates and refines code
4. Review, approve, and iterate on AI-generated specifications and code
5. Access a WebSocket-based terminal (currently disabled for security)
6. Monitor workflow progress via SSE (Server-Sent Events)

## Tech Stack & Dependencies

### Go Gateway (API Layer)
| Dependency | Version | Purpose |
|------------|---------|---------|
| go-chi/chi/v5 | v5.2.3 | HTTP router |
| golang-jwt/jwt/v5 | v5.3.0 | JWT authentication |
| jackc/pgx/v5 | v5.7.1 | PostgreSQL driver |
| redis/go-redis/v9 | v9.17.2 | Redis client |
| go-playground/validator/v10 | v10.29.0 | Request validation |
| pquerna/otp | v1.5.0 | TOTP for MFA |
| prometheus/client_golang | v1.23.2 | Metrics |
| golang.org/x/oauth2 | v0.30.0 | OAuth 2.0 |
| go.opentelemetry.io/otel | v1.39.0 | Tracing |

### Python API (Worker Service)
| Dependency | Version | Purpose |
|------------|---------|---------|
| fastapi | >=0.100.0 | API framework |
| sqlalchemy | >=2.0.0 | ORM |
| asyncpg | >=0.28.0 | Async PostgreSQL |
| crewai | >=0.1.0 | Multi-agent orchestration |
| python-jose | >=3.3.0 | JWT handling |
| passlib | >=1.7.4 | Password hashing |
| redis | >=5.0.0 | Redis client |

### Infrastructure
- PostgreSQL 15+
- Redis 7+
- Node.js 20+ (Frontend)

## Architectural Components

### Data Models (PostgreSQL)

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│    User     │     │   Project   │     │    Task     │
├─────────────┤     ├─────────────┤     ├─────────────┤
│ id          │     │ id          │◄────│ project_id  │
│ email       │     │ name        │     │ title       │
│ username    │     │ description │     │ priority    │
│ password_hash│     │ status      │     │ status      │
│ mfa_enabled │     │ created_by  │     │ crew_run_id │
│ mfa_secret  │     └─────────────┘     └─────────────┘
│ backup_codes│           │                   │
└─────────────┘           ▼                   ▼
      │           ┌─────────────┐     ┌─────────────┐
      │           │  Artifact   │     │  CrewRun    │
      │           ├─────────────┤     ├─────────────┤
      │           │ project_id  │     │ id          │
      │           │ task_id     │     │ crew_id     │
      │           │ name        │     │ status      │
      │           │ type        │     │ input       │
      │           │ content     │     │ result      │
      │           └─────────────┘     └─────────────┘
      │                                     │
      ▼                                     ▼
┌─────────────┐                     ┌───────────────┐
│OAuthAccount │                     │ WorkflowStage │
├─────────────┤                     ├───────────────┤
│ user_id     │                     │ crew_run_id   │
│ provider    │                     │ stage         │
│ provider_uid│                     │ status        │
│ tokens      │                     │ output        │
└─────────────┘                     └───────────────┘
```

### API Endpoints

#### Go Gateway (Port 8001)
| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | /auth/register | No | User registration |
| POST | /auth/login | No | User login |
| GET | /auth/me | Yes | Get current user |
| GET | /auth/oauth/{provider} | No | OAuth redirect |
| GET | /auth/oauth/{provider}/callback | No | OAuth callback |
| POST | /auth/mfa/setup | Yes | Setup MFA |
| POST | /auth/mfa/enable | Yes | Enable MFA |
| POST | /auth/mfa/verify | No | Verify TOTP |
| GET | /auth/sessions | Yes | List sessions |
| DELETE | /auth/sessions | Yes | Revoke sessions |
| GET | /projects | Yes | List projects |
| POST | /projects | Yes | Create project |
| GET | /projects/{id} | Yes | Get project |
| GET | /projects/{id}/dashboard | Yes | Get dashboard |
| POST | /projects/{id}/tasks | Yes | Create task |
| GET | /projects/{id}/tasks | Yes | List tasks |
| GET | /health | No | Health check |
| GET | /admin/providers | No | LLM providers |

#### Python API (Port 8002)
| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | /crews/runs | Yes | Create workflow run |
| GET | /crews/runs/{id} | Yes | Get run status |
| POST | /crews/runs/{id}/cancel | Yes | Cancel run |
| GET | /crews/runs/{id}/events | Yes | SSE events |
| WS | /ws/terminal | Yes | PTY terminal |
| GET | /projects/{id}/workflow/* | Yes | Workflow operations |
| GET | /admin/providers | No | Provider status |

### Key Services/Modules

1. **Go Gateway**
   - `auth/auth.go` - JWT token creation/validation
   - `auth/oauth.go` - OAuth 2.0 with Google, GitHub
   - `auth/mfa.go` - TOTP multi-factor authentication
   - `auth/sessions.go` - Redis session management
   - `handlers/` - HTTP request handlers
   - `middleware/` - Rate limiting, logging

2. **Python API**
   - `agents/` - CrewAI agent definitions (Planner, Coder, Tester, etc.)
   - `workflows/pipeline.py` - Multi-agent orchestration
   - `llm_providers.py` - Multi-cloud LLM abstraction
   - `auth.py` - JWT authentication
   - `middleware/` - Rate limiting, metrics, error handling

### Data Flow Analysis

```
                    ┌──────────────────┐
                    │  React Frontend  │
                    │    (Port 3000)   │
                    └────────┬─────────┘
                             │ REST + SSE + WS
                             ▼
                    ┌──────────────────┐
                    │   Go Gateway     │
                    │    (Port 8001)   │
                    │  ┌────────────┐  │
                    │  │ JWT Auth   │  │
                    │  │ Rate Limit │  │
                    │  │ Sessions   │  │
                    │  └────────────┘  │
                    └────────┬─────────┘
                             │ HTTP Proxy
                             ▼
                    ┌──────────────────┐
                    │  Python Workers  │
                    │    (Port 8002)   │
                    │  ┌────────────┐  │
                    │  │ Planner    │  │
                    │  │ Coder      │──┼──► LLM APIs
                    │  │ Tester     │  │   (OpenRouter/
                    │  └────────────┘  │    Vertex/etc)
                    └────────┬─────────┘
                             │
            ┌────────────────┼────────────────┐
            ▼                ▼                ▼
     ┌───────────┐    ┌───────────┐    ┌───────────┐
     │ PostgreSQL│    │   Redis   │    │ SSE/Events│
     │ (State)   │    │ (Sessions)│    │ (Realtime)│
     └───────────┘    └───────────┘    └───────────┘
```

---

# PHASE 2: Critical Flaw Identification

## A. Security Vulnerability Assessment

### Finding S1: Default JWT Secret in Development

**Severity**: [P1 - HIGH]  
**Location**: [config.go](file:///home/thomas/fullstack-ai-workflow-system/apps/gateway/internal/config/config.go#L86)

```go
JWTSecretKey: getEnv("JWT_SECRET_KEY", "dev-secret-key-change-in-production"),
```

**Analysis**: A default secret key is provided for development. While production validation exists in [main.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/main.py#L110-L114), the same validation doesn't exist at Go gateway startup. If deployed without proper configuration, the default can be used.

**Recommendation**: Add startup validation in Go similar to Python:
```go
if cfg.IsProduction() && cfg.JWTSecretKey == "dev-secret-key-change-in-production" {
    log.Fatal("CRITICAL: JWT_SECRET_KEY must be changed in production")
}
```

---

### Finding S2: OAuth State Store Not Persistent (Race Condition Risk)

**Severity**: [P2 - MEDIUM]  
**Location**: [oauth.go](file:///home/thomas/fullstack-ai-workflow-system/apps/gateway/internal/auth/oauth.go#L271-L295)

```go
type OAuthStateStore struct {
    states map[string]time.Time
}
```

**Analysis**: The OAuth state store uses an in-memory map without mutex protection. This creates:
1. **Race conditions** if multiple requests access concurrently
2. **State loss** on server restart (denial of service for in-progress OAuth flows)
3. **Scalability issues** if multiple gateway instances are deployed

**Recommendation**: Use Redis for state storage (already available) with proper TTL:
```go
func (s *RedisStateStore) Store(ctx context.Context, state string) error {
    return s.client.Set(ctx, "oauth_state:"+state, "1", 10*time.Minute).Err()
}
```

---

### Finding S3: Terminal WebSocket Provides Full Shell Access

**Severity**: [P0 - CRITICAL] (Currently Mitigated)  
**Location**: [main.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/main.py#L291-L478)

```python
@app.websocket("/ws/terminal")
async def terminal_websocket(websocket: WebSocket, token: str = None):
    # ...
    os.execvpe("/bin/bash", ["/bin/bash", "-i"], env)
```

**Analysis**: The terminal endpoint spawns a real bash shell with full access to the host system. This is currently **disabled** via comment (line 94), which is the correct decision. However, the code remains and could be accidentally re-enabled.

**Status**: ✅ Mitigated - Web terminal is disabled

**Recommendation**:
1. Move terminal code to a separate, clearly marked module
2. Require explicit `ENABLE_WEB_TERMINAL=true` environment variable
3. When re-enabled, run shells in isolated containers (Docker/Firecracker)

---

### Finding S4: OAuth Tokens Stored Unencrypted

**Severity**: [P2 - MEDIUM]  
**Location**: [models.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/db/models.py#L205-L206)

```python
access_token = Column(Text(), nullable=True)  # For API calls (encrypted in production)
refresh_token = Column(Text(), nullable=True)  # For token refresh
```

**Analysis**: The comment says "encrypted in production" but no encryption is implemented. OAuth tokens for Google/GitHub are stored in plaintext in the database.

**Recommendation**: Implement field-level encryption using Fernet or similar:
```python
from cryptography.fernet import Fernet
cipher = Fernet(settings.ENCRYPTION_KEY)
encrypted_token = cipher.encrypt(token.encode())
```

---

### Finding S5: Timing Attack on Password Verification

**Severity**: [P2 - MEDIUM]  
**Location**: [handlers.go](file:///home/thomas/fullstack-ai-workflow-system/apps/gateway/internal/handlers/handlers.go#L239-L244)

```go
user, err := h.db.GetUserByEmail(r.Context(), req.Email)
if err != nil || !auth.CheckPassword(req.Password, user.PasswordHash) {
    h.writeError(w, http.StatusUnauthorized, "invalid_credentials", "Incorrect email or password")
    return
}
```

**Analysis**: If `GetUserByEmail` fails (user doesn't exist), the password check is skipped, potentially revealing user existence via timing differences. The bcrypt library's constant-time comparison helps, but the database lookup timing still differs.

**Recommendation**: Always perform password check even for non-existent users:
```go
user, err := h.db.GetUserByEmail(r.Context(), req.Email)
dummyHash := "$2a$10$..."  // Pre-computed hash
if user == nil {
    auth.CheckPassword(req.Password, dummyHash)  // Burn time
    h.writeError(...)
    return
}
```

---

### Finding S6: Missing CSRF Protection on Cookie-Based Auth

**Severity**: [P2 - MEDIUM]  
**Location**: [handlers.go](file:///home/thomas/fullstack-ai-workflow-system/apps/gateway/internal/handlers/handlers.go#L262-L270)

```go
http.SetCookie(w, &http.Cookie{
    Name:     "access_token",
    Value:    accessToken,
    Path:     "/",
    HttpOnly: true,
    Secure:   h.cfg.IsProduction(),
    SameSite: http.SameSiteLaxMode,
    MaxAge:   h.cfg.JWTExpireMinutes * 60,
})
```

**Analysis**: Cookies with `SameSite: Lax` provide basic CSRF protection but are not sufficient for all scenarios. State-changing requests via GET (if any) are still vulnerable. While `SameSite: Strict` would be stronger, it breaks OAuth redirects.

**Recommendation**: Implement double-submit cookie pattern for state-changing operations, or use `SameSite: Strict` after OAuth is complete.

---

## B. Brittleness & Maintainability Analysis

### Finding B1: Duplicate Imports in Pipeline

**Severity**: [P2 - MEDIUM]  
**Location**: [pipeline.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/workflows/pipeline.py#L12-L17)

```python
from ..agents.planner import run_planner, validate_specification
from ..agents.coder import run_coder, validate_code_output, parse_code_output
from ..agents.tester import run_tester, parse_test_output, has_blocking_issues
from ..prompt_processor import prompt_processor
from ..agents.tester import run_tester, parse_test_output, has_blocking_issues  # DUPLICATE
from ..prompt_processor import prompt_processor  # DUPLICATE
```

**Analysis**: Hallmark of context-less LLM generation - exact duplicate imports on consecutive lines.

**Recommendation**: Remove duplicate lines 16-17.

---

### Finding B2: Inconsistent Artifact Metadata Field Name

**Severity**: [P2 - MEDIUM]  
**Location**: [models.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/db/models.py#L181) vs [pipeline.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/workflows/pipeline.py#L478)

**Model**:
```python
meta = Column("metadata", JSONB(...), nullable=True)  # Renamed from metadata (reserved word)
```

**Usage**:
```python
metadata={
    "description": file_obj.get("description", ""),
    "generated_by": "coder_agent"
},
```

**Analysis**: The model defines a Python attribute `meta` mapped to database column `metadata`, but the pipeline uses `metadata=` as a kwarg when constructing Artifact objects. This will silently pass to SQLAlchemy but creates confusing code.

**Recommendation**: Consistently use `meta=` in all code that instantiates Artifact objects.

---

### Finding B3: Hardcoded "default" Crew ID

**Severity**: [P2 - MEDIUM]  
**Location**: [pipeline.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/workflows/pipeline.py#L515)

```python
crew_run = CrewRun(
    id=workflow_id,
    crew_id="default",  # HARDCODED
    status="running",
    ...
)
```

**Analysis**: The crew_id is hardcoded to "default" rather than being configurable or derived from the workflow type.

**Recommendation**: Pass crew_id as a parameter or derive it from the workflow type.

---

### Finding B4: Magic Number for Max Request Body Size

**Severity**: [P2 - MEDIUM]  
**Location**: [handlers.go](file:///home/thomas/fullstack-ai-workflow-system/apps/gateway/internal/handlers/handlers.go#L83)

```go
const maxRequestBodySize = 1 << 20
```

**Analysis**: While documented as a constant, this should be configurable for different deployment scenarios.

**Recommendation**: Add to Config struct with environment variable override.

---

### Finding B5: User ID Confusion in CrewRun

**Severity**: [P2 - MEDIUM]  
**Location**: [pipeline.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/workflows/pipeline.py#L518)

```python
user_id=project_id  # Using project_id as user_id proxy if not provided
```

**Analysis**: The project_id is used as user_id, which is semantically incorrect and will cause confusion when querying runs by user.

**Recommendation**: Pass actual user_id through the workflow or leave null if not available.

---

### Finding B6: active_terminals Dict Uses Wrong Method

**Severity**: [P1 - HIGH] (Runtime Error)  
**Location**: [main.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/main.py#L477)

```python
active_terminals.discard(terminal_id)
```

**Analysis**: `active_terminals` is typed as `dict[str, dict]` but `.discard()` is a set method, not a dict method. This will raise an `AttributeError` at runtime when a terminal disconnects.

**Recommendation**: Use `active_terminals.pop(terminal_id, None)` instead.

---

## C. Failure Route & Resilience Analysis

### Finding R1: No Database Connection Pool Health Checks

**Severity**: [P2 - MEDIUM]  
**Location**: Various

**Analysis**: Neither the Go nor Python services implement periodic health checks for database connection pools. Dead connections in the pool can cause request failures.

**Recommendation**: 
- Go: Configure pgx pool with `HealthCheckPeriod`
- Python: Use SQLAlchemy's `pool_pre_ping=True`

---

### Finding R2: Missing Circuit Breaker for LLM API Calls

**Severity**: [P1 - HIGH]  
**Location**: [llm_providers.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/llm_providers.py)

**Analysis**: LLM API calls have no circuit breaker, retry logic, or timeout configuration. A slow or failing provider can cause request timeouts to cascade.

**Recommendation**: Implement circuit breaker pattern:
```python
from tenacity import retry, stop_after_attempt, wait_exponential
from pybreaker import CircuitBreaker

llm_breaker = CircuitBreaker(fail_max=5, reset_timeout=30)

@llm_breaker
@retry(stop=stop_after_attempt(3), wait=wait_exponential())
async def call_llm(prompt: str): ...
```

---

### Finding R3: Redis Session Manager Returns nil Gracefully

**Severity**: [P2 - MEDIUM] (Documented Behavior)  
**Location**: [sessions.go](file:///home/thomas/fullstack-ai-workflow-system/apps/gateway/internal/auth/sessions.go#L78-L81)

```go
func (m *SessionManager) CreateSession(...) (*Session, error) {
    if m == nil {
        return nil, nil
    }
```

**Analysis**: All SessionManager methods return nil when Redis is unavailable. This is intentional (sessions disabled), but callers must check for nil consistently.

**Status**: ✅ Acceptable - Graceful degradation

---

### Finding R4: Workflow Pipeline Has No Transaction

**Severity**: [P2 - MEDIUM]  
**Location**: [pipeline.py](file:///home/thomas/fullstack-ai-workflow-system/apps/api/app/workflows/pipeline.py#L462-L502)

**Analysis**: The `_store_artifacts` method uses a single session but artifact creation is not atomic. A failure mid-way could result in partial data.

**Recommendation**: Wrap in transaction or use bulk insert with explicit rollback.

---

### Finding R5: No Rate Limiting on MFA Verification

**Severity**: [P1 - HIGH]  
**Location**: [auth_handlers.go](file:///home/thomas/fullstack-ai-workflow-system/apps/gateway/internal/handlers/auth_handlers.go#L208-L263)

**Analysis**: The MFA verification endpoint has no specific rate limiting. An attacker could brute-force the 6-digit TOTP code (1 million combinations) given enough time.

**Recommendation**: Add aggressive rate limiting (5 attempts per 5 minutes) with account lockout.

---

# PHASE 3: Strategic Refactoring Roadmap

## Executive Summary

The **Fullstack AI Workflow System** is a moderately well-architected application with a hybrid Go/Python design that generally follows good practices. The codebase shows evidence of rapid development with occasional inconsistencies typical of multi-contributor or LLM-assisted development.

**Key Strengths**:
- Clean separation of concerns (Gateway/Workers/Frontend)
- Production configuration validation implemented
- JWT auth with proper token types and expiration
- Password complexity requirements enforced
- Terminal WebSocket correctly disabled pending sandboxing
- Good test coverage foundation (8 test files)

**Critical Risks**:
1. OAuth state store race condition / no persistence
2. Missing MFA rate limiting (brute-force vulnerability)
3. Runtime error in terminal cleanup (`dict.discard`)
4. No LLM API resilience (circuit breaker, timeouts)

**Overall Assessment**: **7/10** - Solid foundation with security hardening needed before production deployment.

---

## Prioritized Action Plan

### [P0 - CRITICAL] - Fix Immediately

| ID | Finding | File | Action |
|----|---------|------|--------|
| B6 | `active_terminals.discard()` runtime error | main.py:477 | Change to `.pop(terminal_id, None)` |

### [P1 - HIGH] - Fix Before Production

| ID | Finding | File | Action |
|----|---------|------|--------|
| S1 | Default JWT secret | config.go | Add startup validation |
| R2 | No LLM circuit breaker | llm_providers.py | Add tenacity + pybreaker |
| R5 | No MFA rate limiting | auth_handlers.go | Add 5/5min limit |

### [P2 - MEDIUM] - Fix in Next Sprint

| ID | Finding | File | Action |
|----|---------|------|--------|
| S2 | OAuth state race condition | oauth.go | Move to Redis |
| S4 | Unencrypted OAuth tokens | models.py | Add field encryption |
| S5 | Timing attack on login | handlers.go | Add dummy check |
| S6 | CSRF protection | handlers.go | Double-submit cookie |
| B1 | Duplicate imports | pipeline.py | Remove duplicates |
| B2 | Inconsistent metadata name | pipeline.py | Use `meta=` |
| B3 | Hardcoded crew_id | pipeline.py | Parameterize |
| B5 | project_id as user_id | pipeline.py | Pass real user_id |
| R1 | No DB pool health check | session.py | Enable pool_pre_ping |
| R4 | Non-atomic artifact save | pipeline.py | Add transaction |

---

## Testing & Validation Strategy

### Existing Test Suite
The project has test coverage in `apps/api/tests/`:
- `test_auth.py` - Authentication tests
- `test_security.py` - Security-related tests  
- `test_agents.py` - Agent functionality tests
- `test_workflow_integration.py` - Workflow integration tests
- `test_prompt_processor.py` - Prompt processing tests
- `test_runs.py` - Run management tests

**Run existing tests**:
```bash
cd apps/api
source .venv/bin/activate
pytest tests/ -v --cov=app
```

### Recommended Additional Tests

1. **Rate Limit Testing for MFA** (after implementing)
   ```bash
   # Manual test: Attempt 6 MFA verifications in quick succession
   # Expect 429 after 5th attempt
   ```

2. **OAuth State Persistence Test** (after Redis migration)
   ```bash
   # Test: Restart gateway during OAuth flow, verify state survives
   ```

3. **LLM Circuit Breaker Test** (after implementing)
   ```python
   # Mock LLM to fail 5 times, verify circuit opens
   ```

---

## Documentation Blueprint

### Missing Documentation (Create These)

1. **SECURITY.md** - Security practices, vulnerability reporting
2. **DEPLOYMENT.md** - Production deployment checklist
3. **API.md** - Complete API documentation (OpenAPI spec exists, needs prose)
4. **ARCHITECTURE.md** - High-level architecture decisions

### Existing Documentation (Update These)

1. **README.md** - Add security considerations section
2. **docker-compose.yml** - Add comments for production configuration

---

*End of Report*

**Signed**: Axiom  
**Date**: 2025-12-15
