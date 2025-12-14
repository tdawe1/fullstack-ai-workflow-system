"""
Multi-agent system for prompt-driven code generation.

Hierarchical agent architecture:

SUPERVISORY LAYER:
- Architect: High-level system design, technology decisions
- Planner: Task breakdown, dependency ordering (iterates with Architect)

EXECUTION LAYER:
- Orchestrator: Coordinates execution agents per task
- Coder: Generates code from specifications
- Tester: Reviews code and creates tests

INTEGRATION LAYER:
- Integrator: Implements merges, consults supervisory agents
"""

from .architect import create_architect_agent, run_architect
from .orchestrator import create_orchestrator_agent, create_planning_task, run_orchestrator
from .coder import create_coder_agent, create_coding_task, run_coder
from .tester import create_tester_agent, create_testing_task, run_tester
from .integrator import create_integrator_agent, run_integrator
from .supervisory_crew import run_supervisory_crew


__all__ = [
    # Supervisory
    "create_architect_agent",
    "run_architect",
    "run_supervisory_crew",
    # Execution
    "create_orchestrator_agent",
    "create_planning_task",
    "run_orchestrator",
    "create_coder_agent",
    "create_coding_task",
    "run_coder",
    "create_tester_agent",
    "create_testing_task",
    "run_tester",
    # Integration
    "create_integrator_agent",
    "run_integrator",
]
