import time
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy import text

from ..core.config import settings


engine = create_async_engine(settings.DATABASE_URL, echo=settings.DB_ECHO, future=True)

AsyncSessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)


async def get_session() -> AsyncSession:
    async with AsyncSessionLocal() as session:
        yield session


async def check_db_health() -> tuple[bool, float, str | None]:
    """Check database connection health.
    
    Returns:
        Tuple of (is_healthy, latency_ms, error_message)
        - is_healthy: True if connection succeeded
        - latency_ms: Round-trip time in milliseconds
        - error_message: Error description if unhealthy, None otherwise
    """
    start = time.time()
    try:
        async with AsyncSessionLocal() as session:
            await session.execute(text("SELECT 1"))
            latency_ms = (time.time() - start) * 1000
            return (True, latency_ms, None)
    except Exception as e:
        latency_ms = (time.time() - start) * 1000
        return (False, latency_ms, str(e))

