import os
from pathlib import Path

import pytest
import pytest_asyncio
from alembic import command
from alembic.config import Config
from httpx import ASGITransport, AsyncClient
from sqlalchemy import text

TEST_DB_URL = os.getenv("TEST_DATABASE_URL") or "postgresql+asyncpg://kyros:kyros@localhost:5432/kyros_test"
os.environ.setdefault("DATABASE_URL", TEST_DB_URL)

from app.main import app  # noqa: E402
from app.db.session import AsyncSessionLocal, engine  # noqa: E402


@pytest.fixture(scope="session", autouse=True)
def apply_migrations():
    cfg = Config(str(Path(__file__).resolve().parents[1] / "alembic.ini"))
    cfg.set_main_option("sqlalchemy.url", os.environ["DATABASE_URL"])
    command.upgrade(cfg, "head")
    yield
    if "kyros_test" in os.environ["DATABASE_URL"]:
        command.downgrade(cfg, "base")


@pytest_asyncio.fixture
async def db_cleanup(apply_migrations):
    await engine.dispose()
    async with AsyncSessionLocal() as session:
        # Truncate all tables including users (added for auth tests)
        await session.execute(text("TRUNCATE TABLE crew_events, crew_runs, users RESTART IDENTITY CASCADE"))
        await session.commit()
    yield
    await engine.dispose()


@pytest_asyncio.fixture
async def db_session():
    async with AsyncSessionLocal() as session:
        yield session


@pytest_asyncio.fixture
async def api_client():
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://testserver") as client:
        yield client


@pytest.fixture(autouse=True)
def mock_llm_agents(monkeypatch):
    """Mock LLM agents to prevent live API calls."""
    
    async def mock_run_planner(*args, **kwargs):
        return {
            "status": "completed",
            "specification": {
                "purpose": "Mocked purpose",
                "components": ["mock_comp"],
                "technology": {"language": "python"},
                "file_structure": {},
                "dependencies": []
            },
            "simulation": True
        }
    
    async def mock_run_coder(*args, **kwargs):
        return {
            "status": "completed",
            "code_output": {
                "files": [
                    {"path": "mock.py", "content": "print('mock')"}
                ]
            },
            "files": [
                {"path": "mock.py", "content": "print('mock')"}
            ],
            "simulation": True
        }

    async def mock_run_tester(*args, **kwargs):
        return {
            "status": "completed",
            "test_output": {
                "review": {"matches_spec": True, "issues": []},
                "tests": []
            },
            "review": {"matches_spec": True, "issues": []},
            "tests": [],
            "simulation": True
        }

    async def mock_run_orchestrator(*args, **kwargs):
        return {
            "status": "completed",
            "specification": {
                "purpose": "Mocked orchestrated purpose",
                "phases": []
            },
            "simulation": True
        }

    # Patch Source
    monkeypatch.setattr("app.agents.planner.run_planner", mock_run_planner)
    monkeypatch.setattr("app.agents.coder.run_coder", mock_run_coder)
    monkeypatch.setattr("app.agents.tester.run_tester", mock_run_tester)
    monkeypatch.setattr("app.agents.orchestrator.run_orchestrator", mock_run_orchestrator)
    
    # Patch Consumers (Pipeline)
    # Note: We use setattr on the module object of app.workflows.pipeline
    # We need to import it here to patch it, or use string path if accessible
    monkeypatch.setattr("app.workflows.pipeline.run_planner", mock_run_planner)
    monkeypatch.setattr("app.workflows.pipeline.run_coder", mock_run_coder)
    monkeypatch.setattr("app.workflows.pipeline.run_tester", mock_run_tester)

