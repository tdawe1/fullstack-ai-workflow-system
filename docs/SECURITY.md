# API Security Guide

## Overview

This document covers security considerations for deploying the Kyros Praxis API.

## ⚠️ Critical Configuration

### JWT Secret Key (REQUIRED)

Generate a secure 32+ character secret for JWT signing:

```bash
openssl rand -hex 32
```

Set in `.env`:
```
JWT_SECRET_KEY=your-generated-secret-here
```

> **CRITICAL**: The API will refuse to start in production if this is not set or is too short.

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `JWT_SECRET_KEY` | **Yes** | 32+ char secret for JWT signing |
| `KYROS_ENV` | Yes | `production` for prod deployments |
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `CORS_ALLOW_ORIGINS` | Yes | Comma-separated allowed origins (no wildcards in prod) |
| `REDIS_URL` | Recommended | For token revocation and caching |
| `ENABLE_TERMINAL` | No | Set `true` only if sandboxed |

---

## 🔴 Web Terminal Warning

The `/ws/terminal` endpoint provides a **real bash shell** with API privileges.

### Current Status: DISABLED

The terminal is disabled by default and should only be enabled when:
1. Running in an isolated container with minimal privileges
2. Using network isolation (internal only)
3. Resource limits are enforced (CPU, memory, disk)

### To Enable (Development Only)

```bash
export ENABLE_TERMINAL=true
```

### Security Measures Implemented

- Authentication required before connection
- Concurrent connection limit (`MAX_TERMINAL_CONNECTIONS=50`)
- Explicit opt-in via environment variable
- Production mode disables by default

---

## Authentication & Authorization

### JWT Token Flow

1. **Access Token**: Short-lived (15 min), used for all API requests
2. **Refresh Token**: Long-lived (7 days), used to get new access tokens
3. **Token Types**: Enforced (`access` vs `refresh`) to prevent misuse

### Token Revocation

Tokens can be revoked via Redis blacklist:
- Individual token revocation (logout)
- User-wide revocation (password change, security breach)

### Password Requirements

- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number
- At least one special character

---

## Prompt Injection Mitigation

User input to AI agents uses structured prompts:
- Clear `[SYSTEM INSTRUCTIONS]` delimiters
- `<<<USER_INPUT_START>>>` / `<<<USER_INPUT_END>>>` boundaries
- Delimiter patterns stripped from user input
- Input length limited to 10KB

---

## Rate Limiting

- Default: 100 requests/minute per IP
- Memory-efficient with periodic cleanup
- `Retry-After` header on 429 responses

---

## CSRF Protection (Go Gateway)

Double-submit cookie pattern:
- Token set in cookie on GET requests
- Required in `X-CSRF-Token` header for POST/PUT/DELETE
- Skipped for API clients with `Authorization` header

---

## Production Checklist

- [ ] `JWT_SECRET_KEY` is 32+ characters
- [ ] `KYROS_ENV=production`
- [ ] `CORS_ALLOW_ORIGINS` specifies exact domains (no `*`)
- [ ] `DEBUG=false`
- [ ] Database uses SSL connection
- [ ] Redis configured for token revocation
- [ ] Web terminal is DISABLED (unless sandboxed)
- [ ] TLS/HTTPS enabled on gateway
- [ ] Rate limiting configured appropriately
