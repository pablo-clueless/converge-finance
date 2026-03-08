-- Drop Workflow Schema

-- Drop RLS policies
DROP POLICY IF EXISTS workflow_delegations_entity_isolation ON workflow.delegations;
DROP POLICY IF EXISTS workflow_pending_entity_isolation ON workflow.pending_approvals;
DROP POLICY IF EXISTS workflow_actions_entity_isolation ON workflow.actions;
DROP POLICY IF EXISTS workflow_requests_entity_isolation ON workflow.requests;
DROP POLICY IF EXISTS workflow_steps_entity_isolation ON workflow.workflow_steps;
DROP POLICY IF EXISTS workflow_workflows_entity_isolation ON workflow.workflows;

-- Disable RLS
ALTER TABLE workflow.delegations DISABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.pending_approvals DISABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.actions DISABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.requests DISABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.workflow_steps DISABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.workflows DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_workflow_delegations_updated_at ON workflow.delegations;
DROP TRIGGER IF EXISTS update_workflow_requests_updated_at ON workflow.requests;
DROP TRIGGER IF EXISTS update_workflow_workflows_updated_at ON workflow.workflows;

-- Drop functions
DROP FUNCTION IF EXISTS workflow.generate_request_number(CHAR(26), VARCHAR(10));

-- Drop tables
DROP TABLE IF EXISTS workflow.delegations;
DROP TABLE IF EXISTS workflow.pending_approvals;
DROP TABLE IF EXISTS workflow.actions;
DROP TABLE IF EXISTS workflow.requests;
DROP TABLE IF EXISTS workflow.workflow_steps;
DROP TABLE IF EXISTS workflow.workflows;

-- Drop enums
DROP TYPE IF EXISTS workflow.action_type;
DROP TYPE IF EXISTS workflow.request_status;
DROP TYPE IF EXISTS workflow.step_type;
DROP TYPE IF EXISTS workflow.workflow_status;

-- Drop schema
DROP SCHEMA IF EXISTS workflow;
