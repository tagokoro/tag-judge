CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL,
    provider_user_id text NOT NULL,
    name text NOT NULL,
    email text NOT NULL,
    role text NOT NULL DEFAULT 'user',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_provider_user_id_unique UNIQUE (provider, provider_user_id),
    CONSTRAINT users_role_check CHECK (role IN ('user', 'author', 'admin'))
);

CREATE TABLE problems (
    id bigserial PRIMARY KEY,
    title text NOT NULL,
    statement text NOT NULL,
    time_limit_ms integer NOT NULL,
    memory_limit_kb integer NOT NULL,
    visibility text NOT NULL DEFAULT 'public',
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT problems_visibility_check CHECK (visibility IN ('public', 'private', 'group')),
    CONSTRAINT problems_time_limit_ms_check CHECK (time_limit_ms > 0),
    CONSTRAINT problems_memory_limit_kb_check CHECK (memory_limit_kb > 0)
);

CREATE TABLE test_cases (
    id bigserial PRIMARY KEY,
    problem_id bigint NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    input text NOT NULL,
    expected_output text NOT NULL,
    is_sample boolean NOT NULL DEFAULT false,
    score integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT test_cases_score_check CHECK (score > 0)
);

CREATE TABLE submissions (
    id bigserial PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    problem_id bigint NOT NULL REFERENCES problems(id),
    language text NOT NULL,
    source_code text NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    score integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    judged_at timestamptz,
    CONSTRAINT submissions_language_check CHECK (language IN ('python')),
    CONSTRAINT submissions_status_check CHECK (status IN ('pending', 'running', 'ac', 'wa', 'tle', 'mle', 're', 'ce', 'ie')),
    CONSTRAINT submissions_score_check CHECK (score >= 0)
);

CREATE TABLE submission_results (
    id bigserial PRIMARY KEY,
    submission_id bigint NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    test_case_id bigint NOT NULL REFERENCES test_cases(id),
    status text NOT NULL,
    execution_time_ms integer,
    memory_kb integer,
    stdout text NOT NULL DEFAULT '',
    stderr text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT submission_results_status_check CHECK (status IN ('ac', 'wa', 'tle', 'mle', 're', 'ce', 'ie')),
    CONSTRAINT submission_results_execution_time_ms_check CHECK (execution_time_ms IS NULL OR execution_time_ms >= 0),
    CONSTRAINT submission_results_memory_kb_check CHECK (memory_kb IS NULL OR memory_kb >= 0)
);

CREATE TABLE code_runs (
    id bigserial PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    language text NOT NULL,
    source_code text NOT NULL,
    stdin text NOT NULL DEFAULT '',
    stdout text NOT NULL DEFAULT '',
    stderr text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending',
    execution_time_ms integer,
    memory_kb integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT code_runs_language_check CHECK (language IN ('python')),
    CONSTRAINT code_runs_status_check CHECK (status IN ('pending', 'running', 'ac', 'tle', 're', 'ie')),
    CONSTRAINT code_runs_execution_time_ms_check CHECK (execution_time_ms IS NULL OR execution_time_ms >= 0),
    CONSTRAINT code_runs_memory_kb_check CHECK (memory_kb IS NULL OR memory_kb >= 0)
);

CREATE INDEX problems_visibility_id_idx ON problems (visibility, id);
CREATE INDEX test_cases_problem_id_idx ON test_cases (problem_id);
CREATE INDEX submissions_user_id_created_at_idx ON submissions (user_id, created_at DESC);
CREATE INDEX submissions_problem_id_created_at_idx ON submissions (problem_id, created_at DESC);
CREATE INDEX submission_results_submission_id_idx ON submission_results (submission_id);
CREATE INDEX code_runs_user_id_created_at_idx ON code_runs (user_id, created_at DESC);
