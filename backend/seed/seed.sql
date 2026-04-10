-- Credentials: test@example.com / password123
CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO users (id, name, email, password, created_at)
VALUES (
    'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d',
    'Test User',
    'test@example.com',
    crypt('password123', gen_salt('bf', 12)),
    NOW()
)
ON CONFLICT (email) DO NOTHING;

INSERT INTO projects (id, name, description, owner_id, created_at)
VALUES (
    'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e',
    'TaskFlow MVP',
    'The initial release of the TaskFlow task management system.',
    'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d',
    NOW()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO tasks (id, title, description, status, priority, project_id, assignee_id, due_date, created_at, updated_at)
VALUES
    (
        'c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f',
        'Set up project repository',
        'Initialize the Git repo, add .gitignore, and configure CI.',
        'done',
        'high',
        'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e',
        'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d',
        '2025-04-15',
        NOW(),
        NOW()
    ),
    (
        'd4e5f6a7-b8c9-4d0e-1f2a-3b4c5d6e7f80',
        'Design database schema',
        'Create migration files for users, projects, and tasks tables.',
        'in_progress',
        'high',
        'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e',
        'a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d',
        '2025-04-20',
        NOW(),
        NOW()
    ),
    (
        'e5f6a7b8-c9d0-4e1f-2a3b-4c5d6e7f8091',
        'Implement authentication endpoints',
        'Build register and login endpoints with JWT token generation.',
        'todo',
        'medium',
        'b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e',
        NULL,
        '2025-04-25',
        NOW(),
        NOW()
    )
ON CONFLICT (id) DO NOTHING;
