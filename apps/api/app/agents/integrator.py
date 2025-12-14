"""
Integrator Agent: Implements merges when reviews/PRs are approved.

Integration layer agent that:
- Handles PR/review approvals and implements merges
- Consults Architect and Planner for feedback on complex merges
- Resolves conflicts and ensures consistency
- Creates final deliverable package
"""

import logging
from typing import Dict, Any, Optional, List

try:
    from crewai import Agent, Task, Crew, Process
    CREWAI_AVAILABLE = True
except ImportError:
    CREWAI_AVAILABLE = False
    Agent = Task = Crew = Process = None


logger = logging.getLogger(__name__)


def create_integrator_agent(llm_config: Optional[Dict] = None) -> Optional[Agent]:
    """
    Create the Integrator agent for merge implementation.
    
    Args:
        llm_config: Optional LLM configuration override
        
    Returns:
        Agent instance or None if CrewAI not available
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, cannot create integrator agent")
        return None
    
    return Agent(
        role="Integration Engineer",
        goal="Implement approved merges and create coherent, deployable systems",
        backstory="""You are a senior integration engineer responsible for 
        implementing merges when PRs/reviews are approved. You:
        - Analyze merge requests and identify potential conflicts
        - Consult the Architect for design alignment questions
        - Consult the Planner for implementation sequence guidance
        - Resolve conflicts while maintaining code quality
        - Create comprehensive, deployable packages
        
        You escalate to supervisory agents (Architect/Planner) when you 
        encounter decisions that could impact system design or implementation 
        approach. You never make architectural decisions unilaterally.""",
        verbose=True,
        allow_delegation=True,  # Can consult Architect/Planner
        tools=[],
    )


def create_integration_task(
    architecture: Dict,
    code_outputs: List[Dict],
    test_outputs: List[Dict],
    review_outputs: List[Dict],
    agent: Agent
) -> Optional[Task]:
    """
    Create task for integrating all outputs.
    
    Args:
        architecture: The system architecture from Architect
        code_outputs: Outputs from Coder agent(s)
        test_outputs: Outputs from Tester agent(s)
        review_outputs: Outputs from Reviewer agent(s)
        agent: The integrator agent
        
    Returns:
        Task instance or None if CrewAI not available
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, cannot create integration task")
        return None
    
    return Task(
        description=f"""
        Integrate all outputs into a final, deployable package.
        
        ARCHITECTURE:
        {architecture}
        
        CODE OUTPUTS ({len(code_outputs)} components):
        {code_outputs}
        
        TEST OUTPUTS ({len(test_outputs)} test suites):
        {test_outputs}
        
        REVIEW FEEDBACK ({len(review_outputs)} reviews):
        {review_outputs}
        
        Your integration tasks:
        
        1. CONFLICT RESOLUTION
           - Identify any conflicts between components
           - Resolve import/dependency conflicts
           - Ensure consistent naming conventions
           - Fix any interface mismatches
        
        2. CODE ASSEMBLY
           - Organize files according to architecture
           - Ensure all imports are correct
           - Add any missing __init__.py files
           - Create entry point file(s)
        
        3. TEST INTEGRATION
           - Combine unit tests into test suite
           - Add integration tests for component boundaries
           - Ensure test configuration is correct
           - Verify all tests can run together
        
        4. DOCUMENTATION
           - Create/update README with setup instructions
           - Document API endpoints (if applicable)
           - Add inline documentation where missing
           - Create CHANGELOG entry
        
        5. DEPLOYMENT PACKAGE
           - Create requirements.txt / package.json
           - Add Dockerfile if appropriate
           - Create .env.example with all config vars
           - Add any build/run scripts needed
        
        6. FINAL VALIDATION
           - Verify file structure matches architecture
           - Check all TODOs from reviews are addressed
           - Ensure no placeholder code remains
           - Validate all external dependencies are listed
        
        Output as valid JSON:
        {{
            "files": [
                {{"path": "...", "content": "..."}},
                ...
            ],
            "conflicts_resolved": [...],
            "todos_remaining": [...],
            "deployment_instructions": "...",
            "integration_notes": "..."
        }}
        """,
        expected_output="""Complete integration package in JSON format with all 
        files, resolved conflicts, and deployment instructions.""",
        agent=agent,
    )


async def run_integrator(
    architecture: Dict,
    code_outputs: List[Dict],
    test_outputs: List[Dict],
    review_outputs: List[Dict],
    project_id: Optional[str] = None,
    llm_config: Optional[Dict] = None
) -> Dict[str, Any]:
    """
    Run integrator agent to combine all outputs.
    
    Args:
        architecture: System architecture from Architect
        code_outputs: Outputs from Coder agent(s)
        test_outputs: Outputs from Tester agent(s)
        review_outputs: Outputs from Reviewer agent(s)
        project_id: Optional project ID for tracking
        llm_config: Optional LLM configuration
        
    Returns:
        Dictionary containing:
        - package: The integrated package
        - status: "completed" or "failed"
        - error: Error message if failed
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, using simulation mode")
        return {
            "status": "completed",
            "package": {
                "files": [
                    {"path": "main.py", "content": "# Simulated main file"},
                    {"path": "requirements.txt", "content": "# Dependencies"},
                    {"path": "README.md", "content": "# Project README"}
                ],
                "conflicts_resolved": [],
                "todos_remaining": [],
                "deployment_instructions": "pip install -r requirements.txt && python main.py",
                "note": "Simulation mode. Install CrewAI for real integration."
            },
            "simulation": True
        }
    
    try:
        logger.info(f"Starting integrator agent for project: {project_id}")
        
        agent = create_integrator_agent(llm_config)
        task = create_integration_task(
            architecture, code_outputs, test_outputs, review_outputs, agent
        )
        
        crew = Crew(
            agents=[agent],
            tasks=[task],
            process=Process.sequential,
            verbose=True
        )
        
        result = crew.kickoff()
        
        logger.info(f"Integrator agent completed for project: {project_id}")
        
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
            "package": output,
            "project_id": project_id,
        }
        
    except Exception as e:
        logger.error(f"Error in integrator agent: {e}", exc_info=True)
        return {
            "status": "failed",
            "error": str(e),
            "project_id": project_id,
        }


def validate_package(package: Any) -> tuple[bool, Optional[str]]:
    """
    Validate that an integration package has all required elements.
    
    Args:
        package: The package to validate (dict or JSON string)
        
    Returns:
        Tuple of (is_valid, error_message)
    """
    import json
    
    if isinstance(package, str):
        try:
            package = json.loads(package)
        except json.JSONDecodeError:
            return False, "Package is not valid JSON"
    
    if not isinstance(package, dict):
        return False, "Package must be a dictionary"
    
    if "files" not in package or not isinstance(package["files"], list):
        return False, "Package must contain a 'files' list"
    
    if len(package["files"]) == 0:
        return False, "Package must contain at least one file"
    
    # Validate each file has path and content
    for i, f in enumerate(package["files"]):
        if "path" not in f:
            return False, f"File {i} missing 'path'"
        if "content" not in f:
            return False, f"File {i} missing 'content'"
    
    return True, None
