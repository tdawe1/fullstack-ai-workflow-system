"""
Architect Agent: High-level system design and technology decisions.

Supervisory layer agent that:
- Analyzes requirements for architecture patterns
- Defines system boundaries and components
- Selects technology stack
- Creates design constraints for downstream agents
"""

import logging
from typing import Dict, Any, Optional

try:
    from crewai import Agent, Task, Crew, Process
    CREWAI_AVAILABLE = True
except ImportError:
    CREWAI_AVAILABLE = False
    Agent = Task = Crew = Process = None


logger = logging.getLogger(__name__)


def create_architect_agent(llm_config: Optional[Dict] = None) -> Optional[Agent]:
    """
    Create the Architect agent for high-level system design.
    
    Args:
        llm_config: Optional LLM configuration override
        
    Returns:
        Agent instance or None if CrewAI not available
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, cannot create architect agent")
        return None
    
    return Agent(
        role="Chief Architect",
        goal="Design robust, scalable system architectures that solve complex requirements",
        backstory="""You are a principal software architect with 20+ years of experience 
        designing large-scale distributed systems. You've architected systems at FAANG 
        companies and successful startups. You think in terms of:
        - Scalability and performance requirements
        - Security and compliance constraints  
        - Maintainability and developer experience
        - Cost optimization and cloud-native patterns
        - Domain-driven design and clean architecture
        
        You create architectures that are both pragmatic for immediate needs and 
        extensible for future growth. You document your decisions with clear rationale.""",
        verbose=True,
        allow_delegation=False,
        tools=[],
    )


def create_architecture_task(user_prompt: str, agent: Agent) -> Optional[Task]:
    """
    Create task for generating system architecture.
    
    Args:
        user_prompt: The user's requirements
        agent: The architect agent
        
    Returns:
        Task instance or None if CrewAI not available
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, cannot create architecture task")
        return None
    
    return Task(
        description=f"""
        Design the high-level system architecture for this requirement:
        
        USER REQUIREMENT:
        {user_prompt}
        
        Create a comprehensive architecture document including:
        
        1. SYSTEM OVERVIEW
           - Problem statement and goals
           - Key functional requirements
           - Non-functional requirements (performance, security, scalability)
           - Success metrics
        
        2. ARCHITECTURE PATTERN
           - Selected pattern (microservices, monolith, serverless, etc.)
           - Justification for the choice
           - Trade-offs considered
        
        3. COMPONENT DIAGRAM
           - Major system components
           - Responsibilities of each component
           - Interfaces between components
           - External integrations
        
        4. DATA ARCHITECTURE
           - Data stores and their purposes
           - Data flow between components
           - Caching strategy
           - Event/message patterns
        
        5. TECHNOLOGY STACK
           - Languages and frameworks (with versions)
           - Databases and storage solutions
           - Infrastructure and deployment
           - Third-party services and APIs
           - Justification for each major choice
        
        6. SECURITY ARCHITECTURE
           - Authentication and authorization approach
           - Data protection requirements
           - Network security considerations
           - Compliance requirements
        
        7. SCALABILITY PLAN
           - Expected load and growth
           - Scaling strategies
           - Performance bottlenecks and mitigations
        
        8. CONSTRAINTS & ASSUMPTIONS
           - Technical constraints
           - Business constraints
           - Key assumptions made
        
        Output as valid JSON:
        {{
            "overview": {{...}},
            "pattern": {{...}},
            "components": [...],
            "data_architecture": {{...}},
            "technology_stack": {{...}},
            "security": {{...}},
            "scalability": {{...}},
            "constraints": {{...}}
        }}
        
        This architecture will be passed to the Planner agent for task breakdown.
        """,
        expected_output="""Complete system architecture document in valid JSON format 
        that can be used by the Planner agent to create implementable tasks.""",
        agent=agent,
    )


async def run_architect(
    user_prompt: str,
    project_id: Optional[str] = None,
    llm_config: Optional[Dict] = None
) -> Dict[str, Any]:
    """
    Run architect agent to create high-level system design.
    
    Args:
        user_prompt: The user's requirements
        project_id: Optional project ID for tracking
        llm_config: Optional LLM configuration
        
    Returns:
        Dictionary containing:
        - architecture: The generated architecture (JSON)
        - status: "completed" or "failed"
        - error: Error message if failed
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, using simulation mode")
        return {
            "status": "completed",
            "architecture": {
                "overview": {
                    "problem": f"Architecture for: {user_prompt[:100]}...",
                    "goals": ["functionality", "scalability", "maintainability"]
                },
                "pattern": {
                    "name": "modular_monolith",
                    "justification": "Simple to start, easy to split later"
                },
                "components": [
                    {"name": "api", "type": "http_server"},
                    {"name": "core", "type": "business_logic"},
                    {"name": "storage", "type": "database"}
                ],
                "technology_stack": {
                    "language": "python",
                    "framework": "fastapi",
                    "database": "postgresql"
                },
                "note": "Simulation mode. Install CrewAI for real architecture."
            },
            "simulation": True
        }
    
    try:
        logger.info(f"Starting architect agent for project: {project_id}")
        
        agent = create_architect_agent(llm_config)
        task = create_architecture_task(user_prompt, agent)
        
        crew = Crew(
            agents=[agent],
            tasks=[task],
            process=Process.sequential,
            verbose=True
        )
        
        result = crew.kickoff()
        
        logger.info(f"Architect agent completed for project: {project_id}")
        
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
            "architecture": output,
            "project_id": project_id,
        }
        
    except Exception as e:
        logger.error(f"Error in architect agent: {e}", exc_info=True)
        return {
            "status": "failed",
            "error": str(e),
            "project_id": project_id,
        }


def validate_architecture(arch: Any) -> tuple[bool, Optional[str]]:
    """
    Validate that an architecture has all required sections.
    
    Args:
        arch: The architecture to validate (dict or JSON string)
        
    Returns:
        Tuple of (is_valid, error_message)
    """
    import json
    
    if isinstance(arch, str):
        try:
            arch = json.loads(arch)
        except json.JSONDecodeError:
            return False, "Architecture is not valid JSON"
    
    if not isinstance(arch, dict):
        return False, "Architecture must be a dictionary"
    
    required_sections = [
        "overview",
        "pattern",
        "components",
        "technology_stack"
    ]
    
    missing = [s for s in required_sections if s not in arch]
    
    if missing:
        return False, f"Missing required sections: {', '.join(missing)}"
    
    return True, None
