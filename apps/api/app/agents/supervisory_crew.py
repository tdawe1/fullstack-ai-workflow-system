"""
Supervisory Crew: Architect and Planner deliberation with consensus.

Implements an iterative loop where Architect and Planner refine
the design until they reach consensus on the implementation plan.
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

# Maximum deliberation rounds before forcing consensus
MAX_DELIBERATION_ROUNDS = 3


def create_architect_agent() -> Optional[Agent]:
    """Create the Architect agent for system design."""
    if not CREWAI_AVAILABLE:
        return None
    
    return Agent(
        role="Chief Architect",
        goal="Design robust, scalable system architectures",
        backstory="""You are a principal software architect with 20+ years of experience.
        You think in terms of scalability, security, maintainability, and cost optimization.
        You create clean, pragmatic architectures that are both immediately useful and 
        extensible for future growth.""",
        verbose=True,
        allow_delegation=True,  # Can delegate to Planner
        tools=[],
    )


def create_planner_agent() -> Optional[Agent]:
    """Create the Planner agent for task breakdown."""
    if not CREWAI_AVAILABLE:
        return None
    
    return Agent(
        role="Technical Planner",
        goal="Break down architectures into implementable tasks",
        backstory="""You are an experienced technical project manager who excels at 
        converting high-level designs into concrete, ordered implementation tasks.
        You identify dependencies, estimate effort, and create sprint-ready backlogs.
        You push back on architecture when it's impractical to implement.""",
        verbose=True,
        allow_delegation=True,  # Can delegate back to Architect
        tools=[],
    )


def create_deliberation_tasks(
    user_prompt: str,
    architect: Agent,
    planner: Agent,
    round_num: int = 1,
    previous_architecture: Optional[str] = None,
    previous_feedback: Optional[str] = None
) -> list:
    """
    Create tasks for one round of deliberation.
    
    Args:
        user_prompt: Original user requirements
        architect: The Architect agent
        planner: The Planner agent
        round_num: Current deliberation round
        previous_architecture: Architecture from previous round
        previous_feedback: Planner feedback from previous round
    """
    if not CREWAI_AVAILABLE:
        return []
    
    tasks = []
    
    # Architect task
    if round_num == 1:
        arch_description = f"""
        Design the system architecture for this requirement:
        
        {user_prompt}
        
        Create a comprehensive architecture including:
        - System overview and goals
        - Component breakdown
        - Technology stack with justifications
        - Data architecture
        - Security considerations
        
        Output as structured JSON that the Planner can review.
        """
    else:
        arch_description = f"""
        Revise your architecture based on the Planner's feedback:
        
        ORIGINAL REQUIREMENT: {user_prompt}
        
        YOUR PREVIOUS ARCHITECTURE:
        {previous_architecture}
        
        PLANNER'S FEEDBACK:
        {previous_feedback}
        
        Address the Planner's concerns while maintaining architectural integrity.
        If you disagree with any feedback, explain why in your response.
        
        Output the revised architecture as structured JSON.
        """
    
    architect_task = Task(
        description=arch_description,
        expected_output="System architecture in JSON format",
        agent=architect,
    )
    tasks.append(architect_task)
    
    # Planner task (reviews architecture, provides feedback or approves)
    planner_task = Task(
        description=f"""
        Review the Architect's design for this requirement:
        
        {user_prompt}
        
        Evaluate the architecture for:
        1. IMPLEMENTABILITY: Can this be broken into clear tasks?
        2. COMPLEXITY: Is the scope appropriate?
        3. DEPENDENCIES: Are external dependencies reasonable?
        4. GAPS: Is anything missing for implementation?
        5. RISKS: Are there implementation risks not addressed?
        
        If you APPROVE the architecture:
        - Output: {{"consensus": true, "plan": [...tasks...], "notes": "..."}}
        
        If you have CONCERNS:
        - Output: {{"consensus": false, "concerns": [...], "suggestions": [...]}}
        
        Be constructive. Only raise concerns that materially impact implementation.
        Round {round_num} of {MAX_DELIBERATION_ROUNDS}.
        """,
        expected_output="JSON with consensus status and either plan or concerns",
        agent=planner,
        context=[architect_task],  # Depends on architect output
    )
    tasks.append(planner_task)
    
    return tasks


async def run_supervisory_crew(
    user_prompt: str,
    project_id: Optional[str] = None,
    llm_config: Optional[Dict] = None
) -> Dict[str, Any]:
    """
    Run the Architect/Planner deliberation until consensus.
    
    Args:
        user_prompt: The user's requirements
        project_id: Optional project ID for tracking
        llm_config: Optional LLM configuration
        
    Returns:
        Dictionary containing:
        - architecture: Final agreed architecture
        - plan: Implementation plan from Planner
        - rounds: Number of deliberation rounds
        - status: "completed" or "failed"
    """
    if not CREWAI_AVAILABLE:
        logger.warning("CrewAI not available, using simulation mode")
        return {
            "status": "completed",
            "architecture": {
                "components": ["api", "core", "storage"],
                "technology": {"language": "python", "framework": "fastapi"}
            },
            "plan": [
                {"id": 1, "title": "Set up project structure", "effort": "1h"},
                {"id": 2, "title": "Implement core logic", "effort": "2h"},
                {"id": 3, "title": "Add API endpoints", "effort": "2h"},
                {"id": 4, "title": "Write tests", "effort": "1h"}
            ],
            "rounds": 1,
            "consensus": True,
            "simulation": True
        }
    
    try:
        logger.info(f"Starting supervisory crew for project: {project_id}")
        
        architect = create_architect_agent()
        planner = create_planner_agent()
        
        architecture = None
        feedback = None
        consensus = False
        
        for round_num in range(1, MAX_DELIBERATION_ROUNDS + 1):
            logger.info(f"Deliberation round {round_num}/{MAX_DELIBERATION_ROUNDS}")
            
            tasks = create_deliberation_tasks(
                user_prompt, architect, planner,
                round_num, architecture, feedback
            )
            
            crew = Crew(
                agents=[architect, planner],
                tasks=tasks,
                process=Process.sequential,
                verbose=True
            )
            
            result = crew.kickoff()
            
            # Parse result to check for consensus
            import json
            try:
                if hasattr(result, 'raw'):
                    output = result.raw
                elif hasattr(result, 'tasks_output') and len(result.tasks_output) > 1:
                    # Get planner's output (second task)
                    output = result.tasks_output[1].raw
                else:
                    output = str(result)
                
                # Try to parse planner's response
                if isinstance(output, str):
                    # Find JSON in output
                    import re
                    json_match = re.search(r'\{.*\}', output, re.DOTALL)
                    if json_match:
                        parsed = json.loads(json_match.group())
                        consensus = parsed.get('consensus', False)
                        
                        if consensus:
                            logger.info(f"Consensus reached in round {round_num}")
                            return {
                                "status": "completed",
                                "architecture": architecture or parsed.get('architecture'),
                                "plan": parsed.get('plan', []),
                                "rounds": round_num,
                                "consensus": True,
                                "project_id": project_id
                            }
                        else:
                            feedback = output
                            # Get architecture from first task
                            if hasattr(result, 'tasks_output') and len(result.tasks_output) > 0:
                                architecture = result.tasks_output[0].raw
                
            except json.JSONDecodeError:
                logger.warning(f"Could not parse round {round_num} output as JSON")
                feedback = output
        
        # Max rounds reached - force consensus with last result
        logger.warning("Max deliberation rounds reached, using last result")
        return {
            "status": "completed",
            "architecture": architecture,
            "plan": [],
            "rounds": MAX_DELIBERATION_ROUNDS,
            "consensus": False,
            "forced": True,
            "project_id": project_id
        }
        
    except Exception as e:
        logger.error(f"Error in supervisory crew: {e}", exc_info=True)
        return {
            "status": "failed",
            "error": str(e),
            "project_id": project_id
        }
