"""
Workflow API Routes: HTTP endpoints for agent workflows.

Connects the hierarchical agent system to HTTP endpoints.
"""

import logging
from typing import Optional
from fastapi import APIRouter, HTTPException, BackgroundTasks
from pydantic import BaseModel

from app.agents import (
    run_coder,
    run_tester,
    run_integrator,
    run_supervisory_crew,
)
from app.audit import audit_log

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/workflows", tags=["workflows"])


# Request/Response Models

class WorkflowRequest(BaseModel):
    prompt: str
    project_id: Optional[str] = None
    strategy: Optional[str] = "full"  # full, quick, supervisory_only


class WorkflowResponse(BaseModel):
    workflow_id: str
    status: str
    message: str


class WorkflowStatus(BaseModel):
    workflow_id: str
    phase: str
    status: str
    progress: int
    result: Optional[dict] = None


# In-memory workflow tracking (use Redis in production)
workflows = {}


@router.post("/start", response_model=WorkflowResponse)
async def start_workflow(
    request: WorkflowRequest,
    background_tasks: BackgroundTasks,
):
    """
    Start a new workflow.
    
    Strategies:
    - full: Complete Architect → Planner → Orchestrator → Coder → Tester → Integrator
    - quick: Skip supervisory, go straight to Orchestrator → Coder → Tester
    - supervisory_only: Just Architect/Planner consensus, return plan
    """
    import uuid
    workflow_id = str(uuid.uuid4())
    
    workflows[workflow_id] = {
        "id": workflow_id,
        "status": "started",
        "phase": "initializing",
        "progress": 0,
        "prompt": request.prompt,
        "project_id": request.project_id,
        "strategy": request.strategy,
        "result": None,
    }
    
    audit_log("workflow_started", {
        "workflow_id": workflow_id,
        "project_id": request.project_id,
        "strategy": request.strategy,
    })
    
    # Run workflow in background
    background_tasks.add_task(
        execute_workflow,
        workflow_id,
        request.prompt,
        request.project_id,
        request.strategy,
    )
    
    return WorkflowResponse(
        workflow_id=workflow_id,
        status="started",
        message=f"Workflow started with strategy: {request.strategy}",
    )


@router.get("/status/{workflow_id}", response_model=WorkflowStatus)
async def get_workflow_status(workflow_id: str):
    """Get workflow status."""
    if workflow_id not in workflows:
        raise HTTPException(status_code=404, detail="Workflow not found")
    
    wf = workflows[workflow_id]
    return WorkflowStatus(
        workflow_id=wf["id"],
        phase=wf["phase"],
        status=wf["status"],
        progress=wf["progress"],
        result=wf.get("result"),
    )


@router.get("/list")
async def list_workflows(limit: int = 10):
    """List recent workflows."""
    sorted_wf = sorted(
        workflows.values(),
        key=lambda x: x.get("started_at", 0),
        reverse=True,
    )
    return sorted_wf[:limit]


async def execute_workflow(
    workflow_id: str,
    prompt: str,
    project_id: Optional[str],
    strategy: str,
):
    """Execute the full workflow pipeline."""
    try:
        wf = workflows[workflow_id]
        
        if strategy in ("full", "supervisory_only"):
            # Phase 1: Supervisory
            wf["phase"] = "supervisory"
            wf["progress"] = 10
            
            result = await run_supervisory_crew(prompt, project_id)
            
            if result["status"] != "completed":
                wf["status"] = "failed"
                wf["result"] = {"error": result.get("error", "Supervisory failed")}
                return
            
            architecture = result.get("architecture")
            plan = result.get("plan", [])
            
            wf["progress"] = 30
            
            if strategy == "supervisory_only":
                wf["status"] = "completed"
                wf["phase"] = "done"
                wf["progress"] = 100
                wf["result"] = {"architecture": architecture, "plan": plan}
                audit_log("workflow_completed", {"workflow_id": workflow_id})
                return
        else:
            # Quick mode - skip supervisory
            architecture = None
            plan = [{"task": "Implement: " + prompt[:100]}]
        
        # Phase 2: Execution
        wf["phase"] = "execution"
        wf["progress"] = 40
        
        code_outputs = []
        test_outputs = []
        review_outputs = []
        
        # For each task in plan
        for i, task in enumerate(plan or [{"task": prompt}]):
            task_prompt = task.get("title", task.get("task", prompt))
            
            # Coder
            code_result = await run_coder(task_prompt, project_id)
            code_outputs.append(code_result)
            
            # Tester
            if code_result.get("status") == "completed":
                test_result = await run_tester(
                    code_result.get("code", ""),
                    project_id,
                )
                test_outputs.append(test_result)
            
            wf["progress"] = 40 + int((i + 1) / max(len(plan), 1) * 40)
        
        # Phase 3: Integration
        wf["phase"] = "integration"
        wf["progress"] = 85
        
        integration_result = await run_integrator(
            architecture or {},
            code_outputs,
            test_outputs,
            review_outputs,
            project_id,
        )
        
        wf["progress"] = 100
        wf["status"] = "completed"
        wf["phase"] = "done"
        wf["result"] = integration_result
        
        audit_log("workflow_completed", {
            "workflow_id": workflow_id,
            "phases_completed": ["supervisory", "execution", "integration"],
        })
        
    except Exception as e:
        logger.error(f"Workflow {workflow_id} failed: {e}", exc_info=True)
        workflows[workflow_id]["status"] = "failed"
        workflows[workflow_id]["result"] = {"error": str(e)}
        audit_log("workflow_failed", {"workflow_id": workflow_id, "error": str(e)})
