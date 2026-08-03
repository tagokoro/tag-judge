-- name: CountTestCases :one
SELECT count(*)
FROM test_cases
WHERE problem_id = $1;

-- name: ListTestCases :many
SELECT id, problem_id, input, expected_output, is_sample, score, created_at
FROM test_cases
WHERE problem_id = $1
ORDER BY id;

-- name: GetTestCase :one
SELECT id, problem_id, input, expected_output, is_sample, score, created_at
FROM test_cases
WHERE problem_id = sqlc.arg(problem_id) AND id = sqlc.arg(id);

-- name: CreateTestCase :one
INSERT INTO test_cases (problem_id, input, expected_output, is_sample, score)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, problem_id, input, expected_output, is_sample, score, created_at;

-- name: UpdateTestCase :one
UPDATE test_cases
SET input = sqlc.arg(input),
    expected_output = sqlc.arg(expected_output),
    is_sample = sqlc.arg(is_sample)
WHERE problem_id = sqlc.arg(problem_id) AND id = sqlc.arg(id)
RETURNING id, problem_id, input, expected_output, is_sample, score, created_at;

-- name: DeleteTestCase :execrows
DELETE FROM test_cases
WHERE problem_id = sqlc.arg(problem_id) AND id = sqlc.arg(id);
