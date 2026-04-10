CREATE TABLE IF NOT EXISTS tasks (
    id          UUID PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    status      VARCHAR(20)  NOT NULL DEFAULT 'todo',
    priority    VARCHAR(20)  NOT NULL DEFAULT 'medium',
    project_id  UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    assignee_id UUID         REFERENCES users(id) ON DELETE SET NULL,
    due_date    DATE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT tasks_status_check   CHECK (status IN ('todo', 'in_progress', 'done')),
    CONSTRAINT tasks_priority_check CHECK (priority IN ('low', 'medium', 'high'))
);

CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX idx_tasks_assignee_id ON tasks(assignee_id);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
