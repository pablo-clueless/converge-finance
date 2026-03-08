-- Workflow (Approval) Schema
-- Handles multi-level approval workflows with threshold-based routing, delegation, and escalation

-- Create schema
CREATE SCHEMA IF NOT EXISTS workflow;

-- Enums
CREATE TYPE workflow.workflow_status AS ENUM ('draft', 'active', 'inactive', 'archived');
CREATE TYPE workflow.step_type AS ENUM ('approval', 'notification', 'condition', 'parallel');
CREATE TYPE workflow.request_status AS ENUM ('pending', 'in_progress', 'approved', 'rejected', 'cancelled', 'escalated');
CREATE TYPE workflow.action_type AS ENUM ('approve', 'reject', 'request_info', 'delegate', 'escalate');

-- Workflow Definitions Table
CREATE TABLE workflow.workflows (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    workflow_code VARCHAR(50) NOT NULL,
    workflow_name VARCHAR(100) NOT NULL,
    description TEXT,
    document_type VARCHAR(50) NOT NULL,
    status workflow.workflow_status NOT NULL DEFAULT 'draft',
    version INTEGER NOT NULL DEFAULT 1,
    is_current BOOLEAN NOT NULL DEFAULT true,
    configuration JSONB NOT NULL DEFAULT '{}',
    created_by CHAR(26) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, workflow_code, version)
);

-- Workflow Steps Table
CREATE TABLE workflow.workflow_steps (
    id CHAR(26) PRIMARY KEY,
    workflow_id CHAR(26) NOT NULL REFERENCES workflow.workflows(id) ON DELETE CASCADE,
    step_number INTEGER NOT NULL,
    step_name VARCHAR(100) NOT NULL,
    step_type workflow.step_type NOT NULL DEFAULT 'approval',
    approver_type VARCHAR(20) NOT NULL,
    approver_id CHAR(26),
    approver_expression TEXT,
    threshold_min DECIMAL(18,4),
    threshold_max DECIMAL(18,4),
    threshold_currency CHAR(3) REFERENCES currencies(code),
    required_approvals INTEGER NOT NULL DEFAULT 1,
    allow_self_approval BOOLEAN NOT NULL DEFAULT false,
    escalation_hours INTEGER,
    escalate_to_step INTEGER,
    condition_expression TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workflow_id, step_number),
    CHECK (threshold_min IS NULL OR threshold_max IS NULL OR threshold_min <= threshold_max)
);

-- Approval Requests Table
CREATE TABLE workflow.requests (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    request_number VARCHAR(30) NOT NULL,
    workflow_id CHAR(26) NOT NULL REFERENCES workflow.workflows(id),
    document_type VARCHAR(50) NOT NULL,
    document_id CHAR(26) NOT NULL,
    document_number VARCHAR(50),
    amount DECIMAL(18,4),
    currency_code CHAR(3) REFERENCES currencies(code),
    current_step INTEGER NOT NULL DEFAULT 1,
    status workflow.request_status NOT NULL DEFAULT 'pending',
    requestor_id CHAR(26) NOT NULL,
    requestor_notes TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, request_number)
);

-- Approval Actions Table
CREATE TABLE workflow.actions (
    id CHAR(26) PRIMARY KEY,
    request_id CHAR(26) NOT NULL REFERENCES workflow.requests(id) ON DELETE CASCADE,
    step_id CHAR(26) NOT NULL REFERENCES workflow.workflow_steps(id),
    step_number INTEGER NOT NULL,
    action_type workflow.action_type NOT NULL,
    actor_id CHAR(26) NOT NULL,
    delegated_by CHAR(26),
    comments TEXT,
    acted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Pending Approvals Table (denormalized for performance)
CREATE TABLE workflow.pending_approvals (
    id CHAR(26) PRIMARY KEY,
    request_id CHAR(26) NOT NULL REFERENCES workflow.requests(id) ON DELETE CASCADE,
    step_id CHAR(26) NOT NULL REFERENCES workflow.workflow_steps(id),
    approver_id CHAR(26) NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_at TIMESTAMPTZ,
    reminder_sent BOOLEAN NOT NULL DEFAULT false,
    escalated BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(request_id, step_id, approver_id)
);

-- Delegation Rules Table
CREATE TABLE workflow.delegations (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    delegator_id CHAR(26) NOT NULL,
    delegate_id CHAR(26) NOT NULL,
    workflow_id CHAR(26) REFERENCES workflow.workflows(id),
    document_types TEXT[] DEFAULT '{}',
    start_date DATE NOT NULL,
    end_date DATE,
    reason TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_date IS NULL OR end_date >= start_date),
    CHECK (delegator_id != delegate_id)
);

-- Indexes
CREATE INDEX idx_workflow_workflows_entity ON workflow.workflows(entity_id);
CREATE INDEX idx_workflow_workflows_type ON workflow.workflows(document_type);
CREATE INDEX idx_workflow_workflows_status ON workflow.workflows(status);
CREATE INDEX idx_workflow_workflows_current ON workflow.workflows(entity_id, document_type, is_current) WHERE is_current = true;

CREATE INDEX idx_workflow_steps_workflow ON workflow.workflow_steps(workflow_id);
CREATE INDEX idx_workflow_steps_active ON workflow.workflow_steps(workflow_id, is_active) WHERE is_active = true;

CREATE INDEX idx_workflow_requests_entity ON workflow.requests(entity_id);
CREATE INDEX idx_workflow_requests_workflow ON workflow.requests(workflow_id);
CREATE INDEX idx_workflow_requests_document ON workflow.requests(document_type, document_id);
CREATE INDEX idx_workflow_requests_status ON workflow.requests(status);
CREATE INDEX idx_workflow_requests_requestor ON workflow.requests(requestor_id);
CREATE INDEX idx_workflow_requests_pending ON workflow.requests(entity_id, status) WHERE status IN ('pending', 'in_progress');

CREATE INDEX idx_workflow_actions_request ON workflow.actions(request_id);
CREATE INDEX idx_workflow_actions_actor ON workflow.actions(actor_id);
CREATE INDEX idx_workflow_actions_step ON workflow.actions(step_id);

CREATE INDEX idx_workflow_pending_request ON workflow.pending_approvals(request_id);
CREATE INDEX idx_workflow_pending_approver ON workflow.pending_approvals(approver_id);
CREATE INDEX idx_workflow_pending_due ON workflow.pending_approvals(due_at) WHERE due_at IS NOT NULL;

CREATE INDEX idx_workflow_delegations_entity ON workflow.delegations(entity_id);
CREATE INDEX idx_workflow_delegations_delegator ON workflow.delegations(delegator_id);
CREATE INDEX idx_workflow_delegations_delegate ON workflow.delegations(delegate_id);
CREATE INDEX idx_workflow_delegations_active ON workflow.delegations(is_active, start_date, end_date) WHERE is_active = true;

-- Triggers
CREATE TRIGGER update_workflow_workflows_updated_at
    BEFORE UPDATE ON workflow.workflows
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workflow_requests_updated_at
    BEFORE UPDATE ON workflow.requests
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workflow_delegations_updated_at
    BEFORE UPDATE ON workflow.delegations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Request Number Generation Function
CREATE OR REPLACE FUNCTION workflow.generate_request_number(p_entity_id CHAR(26), p_prefix VARCHAR(10))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_year TEXT;
    v_sequence INTEGER;
    v_request_number VARCHAR(30);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COALESCE(MAX(
        CAST(SUBSTRING(request_number FROM LENGTH(p_prefix) + 6 FOR 6) AS INTEGER)
    ), 0) + 1
    INTO v_sequence
    FROM workflow.requests
    WHERE entity_id = p_entity_id
    AND request_number LIKE p_prefix || v_year || '%';

    v_request_number := p_prefix || v_year || '-' || LPAD(v_sequence::TEXT, 6, '0');

    RETURN v_request_number;
END;
$$ LANGUAGE plpgsql;

-- RLS Policies
ALTER TABLE workflow.workflows ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.workflow_steps ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.pending_approvals ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflow.delegations ENABLE ROW LEVEL SECURITY;

CREATE POLICY workflow_workflows_entity_isolation ON workflow.workflows
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY workflow_steps_entity_isolation ON workflow.workflow_steps
    USING (workflow_id IN (
        SELECT id FROM workflow.workflows
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY workflow_requests_entity_isolation ON workflow.requests
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY workflow_actions_entity_isolation ON workflow.actions
    USING (request_id IN (
        SELECT id FROM workflow.requests
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY workflow_pending_entity_isolation ON workflow.pending_approvals
    USING (request_id IN (
        SELECT id FROM workflow.requests
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY workflow_delegations_entity_isolation ON workflow.delegations
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));
