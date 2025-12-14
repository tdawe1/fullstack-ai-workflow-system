export interface Specification {
  purpose: string;
  components: string[];
  technology: Record<string, unknown>;
  file_structure: Record<string, unknown>;
  dependencies: string[];
  data_models?: Record<string, unknown>;
  implementation_plan?: string[];
  testing_considerations?: string[];
  challenges?: string[];
}

export interface CodeFile {
  id: string;
  name: string;
  content: string;
  metadata: {
    description: string;
    generated_by: string;
  };
  created_at: string;
}

export type ReviewIssueSeverity = 'critical' | 'high' | 'medium' | 'low';

export interface CodeReviewIssue {
  severity: ReviewIssueSeverity;
  description: string;
  suggestion: string;
}

export type CodeReviewQuality = 'excellent' | 'good' | 'fair' | 'poor';

export interface CodeReview {
  matches_spec: boolean;
  overall_quality: CodeReviewQuality;
  issues: CodeReviewIssue[];
}

export interface WorkflowResult {
  project_id: string;
  workflow_id: string;
  status: 'awaiting_approval' | 'completed' | 'needs_refinement' | 'failed';
  stage?: string;
  specification?: Specification;
  code_files?: CodeFile[];
  test_files?: CodeFile[];
  review?: CodeReview;
  message: string;
  iteration?: number;
  validation_score?: number;
}

export interface WorkflowGenerateRequest {
  prompt: string;
}

export interface WorkflowApproveRequest {
  approved: boolean;
  specification: Specification;
}

export interface WorkflowRefineRequest {
  refinement_notes: string;
}

export interface WorkflowStatusResponse {
  project_id: string;
  project_status: 'planning' | 'generating' | 'completed' | 'failed';
  stages: Array<{
    stage: 'planner' | 'coder' | 'tester';
    status: 'active' | 'completed' | 'failed';
    started_at: string;
    completed_at: string | null;
  }>;
}

export interface ProjectSpecificationResponse {
  project_id: string;
  specification: Specification;
  created_at: string;
  status: 'completed' | 'failed';
}

export interface ProjectCodeResponse {
  project_id: string;
  code_files: CodeFile[];
  test_files: CodeFile[];
  total_files: number;
}

export interface ProjectSummary {
  id: string;
  name: string;
  description?: string;
  created_at: string;
  updated_at: string;
  workflow_status: 'idle' | 'planning' | 'generating' | 'awaiting_approval' | 'completed' | 'failed';
}

