"""
Planner Agent: Breaks down architecture into specific implementation tasks.

Supervisory layer agent that:
- Analyzes the architecture and requirements
- Creates a step-by-step implementation plan
- Defines individual tasks for the Coder agent
- Identifies dependencies between tasks
"""

import logging
from typing import Dict, Any, Optional, List, Tuple

try:
    from crewai import Agent, Task, Crew, Process
    CREWAI_AVAILABLE = True
except ImportError:
    CREWAI_AVAILABLE = False
    Agent = Task = Crew = Process = None


logger = logging.getLogger(__name__)


def create_planner_agent(llm_config: Optional[Dict] = None) -> Optional[Agent]:
    """
    Create the Planner agent for task breakdown.
    
    Args:
        llm_config: Optional LLM configuration override
        
    Returns:
        Agent instance or None if CrewAI not available
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, cannot create planner agent")
        return None
    
    return Agent(
        role="Technical Project Manager",
        goal="Break down complex software architectures into manageable, clear implementation tasks",
        backstory="""You are an expert Technical Project Manager and Systems Analyst. 
        You take high-level architectural designs and user requirements and break them down 
        into specific, actionable coding tasks. You understand:
        - Software development lifecycles
        - Dependency management
        - Agile task estimation
        - Technical documentation
        
        Your output is used directly by developers (the Coder agent), so precision and 
        clarity are paramount. You ensure no component is left without an implementation task.""",
        verbose=True,
        allow_delegation=False,
        tools=[],
    )


def create_planning_task(user_prompt: str, architecture: Dict[str, Any], agent: Agent) -> Optional[Task]:
    """
    Create task for generating implementation plan.
    
    Args:
        user_prompt: The user's original requirements
        architecture: The system architecture from the Architect agent
        agent: The planner agent
        
    Returns:
        Task instance or None if CrewAI not available
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, cannot create planning task")
        return None
    
    return Task(
        description=f"""
        Create a detailed implementation plan based on the architecture and requirements.
        
        USER REQUIREMENT:
        {user_prompt}
        
        SYSTEM ARCHITECTURE:
        {architecture}
        
        Break this down into a list of specific coding tasks. For each task, provide:
        1. Title: Concise summary
        2. Description: Detailed instructions for the developer
        3. Files: Which files to create or modify (if known)
        4. Dependencies: Which other tasks must be done first
        5. Complexity: Estimated complexity (Low/Medium/High)
        
        Ensure you cover:
        - Project setup and configuration
        - Database schema and migrations
        - Backend API implementation
        - Frontend components and pages
        - Integration and testing
        
        Output as valid JSON:
        {{
            "plan_overview": "Summary of the approach",
            "phases": [
                {{
                    "name": "Phase 1: Setup",
                    "tasks": [
                        {{
                            "id": "task_1",
                            "title": "Initialize Project",
                            "description": "...",
                            "files": ["pyproject.toml", "Dockerfile"],
                            "complexity": "Low"
                        }}
                    ]
                }}
            ],
            "estimated_timeline": "2 weeks"
        }}
        """,
        expected_output="""Complete implementation plan in valid JSON format containing phases and tasks.""",
        agent=agent,
    )


async def run_planner(
    user_prompt: str,
    project_id: Optional[str] = None,
    architecture: Optional[Dict] = None,
    llm_config: Optional[Dict] = None
) -> Dict[str, Any]:
    """
    Run planner agent to create implementation plan.
    
    Args:
        user_prompt: The user's requirements
        project_id: Optional project ID
        architecture: Optional architecture input (if not provided, simulation prompt used)
        llm_config: Optional LLM configuration
        
    Returns:
        Dictionary containing:
        - specification: The generated plan/spec (JSON)
        - status: "completed" or "failed"
        - error: Error message if failed
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, using simulation mode")
        return {
            "status": "completed",
            "specification": {
                "purpose": f"Implementation plan for: {user_prompt[:50]}...",
                "phases": [
                    {
                        "name": "Phase 1: Foundation",
                        "tasks": [
                            {
                                "id": "t1", 
                                "title": "Setup repository",
                                "description": "Initialize git and dependencies"
                            }
                        ]
                    }
                ],
                "simulation": True
            },
            "project_id": project_id
        }
    
    try:
        logger.info(f"Starting planner agent for project: {project_id}")
        
        agent = create_planner_agent(llm_config)
        task = create_planning_task(user_prompt, architecture or {}, agent)
        
        crew = Crew(
            agents=[agent],
            tasks=[task],
            process=Process.sequential,
            verbose=True
        )
        
        result = crew.kickoff()
        
        logger.info(f"Planner agent completed for project: {project_id}")
        
        # Extract output
        if hasattr(result, 'raw'):
            output = result.raw
        elif hasattr(result, 'output'):
            output = result.output
        elif isinstance(result, dict):
            output = result.get('output', result)
        else:
            output = str(result)
        
        return {
            "status": "completed",
            "specification": output,
            "project_id": project_id,
        }
        
    except Exception as e:
        logger.error(f"Error in planner agent: {e}", exc_info=True)
        return {
            "status": "failed",
            "error": str(e),
            "project_id": project_id,
        }


def validate_specification(spec: Any) -> Tuple[bool, Optional[str]]:
    """
    Validate that a specification/plan has required structure.
    
    Args:
        spec: The specification to validate (dict or JSON string)
        
    Returns:
        Tuple of (is_valid, error_message)
    """
    import json
    
    if isinstance(spec, str):
        try:
            spec = json.loads(spec)
        except json.JSONDecodeError:
            return False, "Specification is not valid JSON"
    
    if not isinstance(spec, dict):
        return False, "Specification must be a dictionary"
    
    # Check for basic plan structure OR simple simulation structure
    if "phases" in spec:
        if not isinstance(spec["phases"], list):
             return False, "Phases must be a list"
    elif "purpose" in spec:
        # Minimal simulation spec validation
        pass
    else:
        return False, "Specification missing 'phases' or 'purpose'"
    
    return True, None
