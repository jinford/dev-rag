-- name: CreateAction :one
INSERT INTO action_backlog (action_id, prompt_version, priority, action_type, title, description, linked_files, owner_hint, acceptance_criteria, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetAction :one
SELECT * FROM action_backlog
WHERE id = $1;

-- name: GetActionByActionID :one
SELECT * FROM action_backlog
WHERE action_id = $1;

-- name: ListActions :many
SELECT * FROM action_backlog
WHERE
    (@priority::text IS NULL OR priority = @priority) AND
    (@action_type::text IS NULL OR action_type = @action_type) AND
    (@status::text IS NULL OR status = @status)
ORDER BY
    CASE priority
        WHEN 'P1' THEN 1
        WHEN 'P2' THEN 2
        WHEN 'P3' THEN 3
    END,
    created_at DESC
LIMIT COALESCE(@limit_count, 100);

-- name: ListPendingActions :many
SELECT * FROM action_backlog
WHERE status = 'open' AND completed_at IS NULL
ORDER BY
    CASE priority
        WHEN 'P1' THEN 1
        WHEN 'P2' THEN 2
        WHEN 'P3' THEN 3
    END,
    created_at DESC;

-- name: UpdateActionStatus :one
UPDATE action_backlog
SET status = $2, completed_at = $3
WHERE id = $1
RETURNING *;

-- name: DeleteAction :exec
DELETE FROM action_backlog
WHERE id = $1;

-- name: CountActionsByStatus :one
SELECT status, COUNT(*) as count
FROM action_backlog
GROUP BY status
ORDER BY
    CASE status
        WHEN 'open' THEN 1
        WHEN 'noop' THEN 2
        WHEN 'completed' THEN 3
    END;

-- name: CountActionsByPriority :one
SELECT priority, COUNT(*) as count
FROM action_backlog
WHERE status = $1
GROUP BY priority
ORDER BY
    CASE priority
        WHEN 'P1' THEN 1
        WHEN 'P2' THEN 2
        WHEN 'P3' THEN 3
    END;

-- name: ListActionsByPriority :many
SELECT * FROM action_backlog
WHERE priority = $1
ORDER BY created_at DESC;

-- name: ListActionsByType :many
SELECT * FROM action_backlog
WHERE action_type = $1
ORDER BY created_at DESC;

-- name: ListActionsByStatus :many
SELECT * FROM action_backlog
WHERE status = $1
ORDER BY
    CASE priority
        WHEN 'P1' THEN 1
        WHEN 'P2' THEN 2
        WHEN 'P3' THEN 3
    END,
    created_at DESC;
