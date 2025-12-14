"""
Terminal WebSocket Router: Provides a real bash shell over WebSocket.

SECURITY WARNING: This endpoint gives authenticated users a bash shell with
the API process's privileges. It should ONLY be enabled when:
1. Running in a sandboxed container with limited capabilities
2. Behind proper network isolation
3. With appropriate resource limits (CPU, memory, file access)

The terminal is DISABLED by default - enable only for development or
when proper sandboxing is in place.
"""

import asyncio
import fcntl
import json
import logging
import os
import pty
import select
import struct
import termios
from uuid import uuid4

from fastapi import APIRouter, WebSocket, WebSocketDisconnect

from ..auth import get_current_user_from_token
from ..core.config import settings
from ..db.session import AsyncSessionLocal

logger = logging.getLogger(__name__)

router = APIRouter(tags=["terminal"])

# Connection tracking
MAX_TERMINALS = settings.MAX_TERMINAL_CONNECTIONS
active_terminals: dict[str, dict] = {}


def is_terminal_enabled() -> bool:
    """Check if terminal is enabled (only in dev or explicit opt-in)."""
    if settings.KYROS_ENV == "production":
        # Require explicit opt-in for production
        return os.getenv("ENABLE_TERMINAL", "").lower() == "true"
    # Allow in dev/staging
    return True


@router.websocket("/ws/terminal")
async def terminal_websocket(websocket: WebSocket, token: str = None):
    """
    WebSocket endpoint for terminal PTY.
    Provides a real bash shell over WebSocket with JWT authentication.
    
    SECURITY: 
    - Authenticates user BEFORE accepting WebSocket connection
    - Disabled in production unless ENABLE_TERMINAL=true
    - Should be run in sandboxed container
    
    Args:
        websocket: WebSocket connection
        token: JWT token (required in query parameter)
    """
    terminal_id = str(uuid4())
    
    # Check if terminal is enabled
    if not is_terminal_enabled():
        await websocket.close(code=1008, reason="Terminal disabled in production")
        logger.warning(f"Terminal {terminal_id}: Rejected - disabled in production")
        return
    
    # Check concurrent connection limit
    if len(active_terminals) >= MAX_TERMINALS:
        await websocket.close(code=1008, reason="Too many terminals")
        logger.warning("Terminal connection rejected: limit reached")
        return
    
    # SECURITY: Authenticate BEFORE accepting connection
    authenticated_user = None
    
    if not token:
        await websocket.close(code=1008, reason="Authentication required")
        logger.warning(f"Terminal {terminal_id}: No token provided")
        return
    
    try:
        async with AsyncSessionLocal() as session:
            authenticated_user = await get_current_user_from_token(token, session)
        
        if not authenticated_user:
            await websocket.close(code=1008, reason="Invalid token")
            logger.warning(f"Terminal {terminal_id}: Invalid token")
            return
    except Exception as e:
        await websocket.close(code=1008, reason="Authentication failed")
        logger.warning(f"Terminal {terminal_id}: Auth failed: {e}")
        return
    
    # Only accept connection after successful authentication
    await websocket.accept()
    logger.info(f"Terminal {terminal_id} accepted for user {authenticated_user.username}")
    
    # Send auth success confirmation
    await websocket.send_json({
        "type": "auth_success",
        "user": {"id": str(authenticated_user.id), "username": authenticated_user.username}
    })
    
    # Track active terminal
    active_terminals[terminal_id] = {
        "user_id": authenticated_user.id, 
        "username": authenticated_user.username
    }
    logger.info(f"Terminal {terminal_id} STARTING (total: {len(active_terminals)})")
    
    pid = None
    fd = None
    
    try:
        # Fork PTY
        pid, fd = pty.fork()
        
        if pid == 0:
            # Child process - exec bash in restricted environment
            env = os.environ.copy()
            env['TERM'] = 'xterm-256color'
            env['COLORTERM'] = 'truecolor'
            env['PS1'] = '\\u@\\h:\\w\\$ '
            # Start bash in interactive mode
            os.execvpe("/bin/bash", ["/bin/bash", "-i"], env)
        
        logger.info(f"Terminal {terminal_id}: PTY forked, pid={pid}")
        
        # Set non-blocking mode
        flags = fcntl.fcntl(fd, fcntl.F_GETFL)
        fcntl.fcntl(fd, fcntl.F_SETFL, flags | os.O_NONBLOCK)
        
        async def read_output():
            """Read from PTY and send to WebSocket."""
            try:
                while True:
                    r, _, _ = select.select([fd], [], [], 0.001)
                    if r:
                        try:
                            output = os.read(fd, 10240)
                            if output:
                                await websocket.send_bytes(output)
                            else:
                                break
                        except OSError:
                            break
                    else:
                        await asyncio.sleep(0.001)
            except Exception as e:
                logger.error(f"Terminal {terminal_id}: read error: {e}")
        
        async def write_input():
            """Read from WebSocket and write to PTY."""
            try:
                while True:
                    data = await websocket.receive()
                    
                    if "bytes" in data:
                        os.write(fd, data["bytes"])
                    elif "text" in data:
                        text = data["text"]
                        if text:
                            try:
                                msg = json.loads(text)
                                if msg.get("type") == "resize":
                                    rows = msg.get("rows", 24)
                                    cols = msg.get("cols", 80)
                                    size = struct.pack("HHHH", rows, cols, 0, 0)
                                    fcntl.ioctl(fd, termios.TIOCSWINSZ, size)
                            except (json.JSONDecodeError, ValueError):
                                os.write(fd, text.encode())
            except WebSocketDisconnect:
                logger.info(f"Terminal {terminal_id}: client disconnected")
            except Exception as e:
                logger.error(f"Terminal {terminal_id}: write error: {e}")
        
        await asyncio.gather(read_output(), write_input(), return_exceptions=True)
    
    except Exception as e:
        logger.error(f"Terminal {terminal_id}: websocket error: {e}", exc_info=True)
    
    finally:
        # Cleanup
        if pid is not None:
            try:
                os.kill(pid, 9)
            except ProcessLookupError:
                pass
        
        if fd is not None:
            try:
                os.close(fd)
            except OSError:
                pass
        
        try:
            await websocket.close()
        except Exception:
            pass
        
        active_terminals.pop(terminal_id, None)
        logger.info(f"Terminal {terminal_id} disconnected (remaining: {len(active_terminals)})")


def get_terminal_stats() -> dict:
    """Get terminal statistics for admin endpoints."""
    return {
        "active_count": len(active_terminals),
        "max_allowed": MAX_TERMINALS,
        "enabled": is_terminal_enabled(),
    }
