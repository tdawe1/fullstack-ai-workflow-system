"""
Audit Logging: Structured security audit trail.

Provides:
- Immutable audit log for security events
- Structured JSON format for analysis
- Support for compliance requirements
"""

import json
import logging
import time
from typing import Any, Dict, Optional
from enum import Enum
from dataclasses import dataclass, asdict
from datetime import datetime


# Configure audit logger
audit_logger = logging.getLogger("audit")
audit_logger.setLevel(logging.INFO)

# Ensure audit logs go to a dedicated file
try:
    handler = logging.FileHandler("audit.log")
    handler.setFormatter(logging.Formatter('%(message)s'))
    audit_logger.addHandler(handler)
except Exception:
    pass  # Fallback to stdout if file not writable


class AuditEventType(Enum):
    # Authentication
    AUTH_LOGIN = "auth.login"
    AUTH_LOGOUT = "auth.logout"
    AUTH_FAILED = "auth.failed"
    AUTH_MFA_ENABLED = "auth.mfa_enabled"
    AUTH_MFA_DISABLED = "auth.mfa_disabled"
    AUTH_PASSWORD_CHANGED = "auth.password_changed"
    AUTH_SESSION_REVOKED = "auth.session_revoked"
    
    # Authorization
    AUTHZ_ACCESS_GRANTED = "authz.access_granted"
    AUTHZ_ACCESS_DENIED = "authz.access_denied"
    
    # Data
    DATA_CREATED = "data.created"
    DATA_READ = "data.read"
    DATA_UPDATED = "data.updated"
    DATA_DELETED = "data.deleted"
    
    # Workflow
    WORKFLOW_STARTED = "workflow.started"
    WORKFLOW_COMPLETED = "workflow.completed"
    WORKFLOW_FAILED = "workflow.failed"
    
    # Admin
    ADMIN_CONFIG_CHANGED = "admin.config_changed"
    ADMIN_USER_CREATED = "admin.user_created"
    ADMIN_USER_DELETED = "admin.user_deleted"
    
    # Security
    SECURITY_RATE_LIMITED = "security.rate_limited"
    SECURITY_SUSPICIOUS = "security.suspicious"


@dataclass
class AuditEvent:
    """Structured audit event."""
    timestamp: str
    event_type: str
    actor: Optional[str]  # User ID or "system"
    resource: Optional[str]  # Resource being accessed
    action: str
    outcome: str  # success, failure, error
    details: Dict[str, Any]
    ip_address: Optional[str] = None
    user_agent: Optional[str] = None
    request_id: Optional[str] = None
    
    def to_json(self) -> str:
        return json.dumps(asdict(self), default=str)


def audit_log(
    event_type: str,
    details: Dict[str, Any],
    actor: Optional[str] = None,
    resource: Optional[str] = None,
    outcome: str = "success",
    ip_address: Optional[str] = None,
    user_agent: Optional[str] = None,
    request_id: Optional[str] = None,
):
    """
    Log an audit event.
    
    Args:
        event_type: Type of event (use AuditEventType or string)
        details: Event-specific details
        actor: User ID or "system"
        resource: Resource being accessed
        outcome: success, failure, or error
        ip_address: Client IP
        user_agent: Client user agent
        request_id: Request correlation ID
    """
    event = AuditEvent(
        timestamp=datetime.utcnow().isoformat() + "Z",
        event_type=event_type if isinstance(event_type, str) else event_type.value,
        actor=actor,
        resource=resource,
        action=event_type.split(".")[-1] if "." in event_type else event_type,
        outcome=outcome,
        details=details,
        ip_address=ip_address,
        user_agent=user_agent,
        request_id=request_id,
    )
    
    audit_logger.info(event.to_json())
    
    # Also emit to metrics if available
    try:
        from app.metrics import audit_events_total
        audit_events_total.labels(event_type=event.event_type, outcome=outcome).inc()
    except ImportError:
        pass


# Convenience functions

def audit_auth_success(user_id: str, method: str, ip: str):
    """Log successful authentication."""
    audit_log(
        AuditEventType.AUTH_LOGIN.value,
        {"method": method},
        actor=user_id,
        ip_address=ip,
    )


def audit_auth_failure(email: str, reason: str, ip: str):
    """Log failed authentication."""
    audit_log(
        AuditEventType.AUTH_FAILED.value,
        {"email": email, "reason": reason},
        actor="anonymous",
        outcome="failure",
        ip_address=ip,
    )


def audit_access_denied(user_id: str, resource: str, action: str, ip: str):
    """Log access denied."""
    audit_log(
        AuditEventType.AUTHZ_ACCESS_DENIED.value,
        {"attempted_action": action},
        actor=user_id,
        resource=resource,
        outcome="failure",
        ip_address=ip,
    )


def audit_data_change(
    user_id: str,
    resource: str,
    action: str,
    entity_id: str,
    changes: Optional[Dict] = None,
):
    """Log data modification."""
    event_map = {
        "create": AuditEventType.DATA_CREATED,
        "read": AuditEventType.DATA_READ,
        "update": AuditEventType.DATA_UPDATED,
        "delete": AuditEventType.DATA_DELETED,
    }
    event_type = event_map.get(action, AuditEventType.DATA_UPDATED)
    
    audit_log(
        event_type.value,
        {"entity_id": entity_id, "changes": changes},
        actor=user_id,
        resource=resource,
    )
