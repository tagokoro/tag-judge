PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'author', 'admin')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (provider, provider_user_id)
);

CREATE TABLE problems (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    statement TEXT NOT NULL,
    time_limit_ms INTEGER NOT NULL CHECK (time_limit_ms > 0),
    memory_limit_kb INTEGER NOT NULL CHECK (memory_limit_kb > 0),
    visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private', 'group')),
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE test_cases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    problem_id INTEGER NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    input TEXT NOT NULL,
    expected_output TEXT NOT NULL,
    is_sample INTEGER NOT NULL DEFAULT 0 CHECK (is_sample IN (0, 1)),
    score INTEGER NOT NULL DEFAULT 1 CHECK (score > 0),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users(id),
    problem_id INTEGER NOT NULL REFERENCES problems(id),
    language TEXT NOT NULL CHECK (language IN ('python')),
    source_code_key TEXT NOT NULL CHECK (trim(source_code_key) <> ''),
    test_cases_key TEXT NOT NULL CHECK (trim(test_cases_key) <> ''),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'ac', 'wa', 'tle', 'mle', 're', 'ce', 'ie')),
    score INTEGER NOT NULL DEFAULT 0 CHECK (score >= 0),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    judged_at TEXT
);

CREATE TABLE submission_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    submission_id INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    test_case_id INTEGER NOT NULL REFERENCES test_cases(id),
    status TEXT NOT NULL CHECK (status IN ('ac', 'wa', 'tle', 'mle', 're', 'ce', 'ie')),
    execution_time_ms INTEGER CHECK (execution_time_ms IS NULL OR execution_time_ms >= 0),
    memory_kb INTEGER CHECK (memory_kb IS NULL OR memory_kb >= 0),
    stdout TEXT NOT NULL DEFAULT '',
    stderr TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (submission_id, test_case_id)
);

CREATE TABLE code_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL REFERENCES users(id),
    language TEXT NOT NULL CHECK (language IN ('python')),
    source_code_key TEXT NOT NULL CHECK (trim(source_code_key) <> ''),
    stdin TEXT NOT NULL DEFAULT '',
    stdout TEXT NOT NULL DEFAULT '',
    stderr TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'ac', 'tle', 're', 'ie')),
    execution_time_ms INTEGER CHECK (execution_time_ms IS NULL OR execution_time_ms >= 0),
    memory_kb INTEGER CHECK (memory_kb IS NULL OR memory_kb >= 0),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT
);

CREATE TABLE groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (trim(name) <> ''),
    description TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE group_members (
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'owner')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX problems_visibility_id_idx ON problems (visibility, id);
CREATE INDEX test_cases_problem_id_idx ON test_cases (problem_id);
CREATE INDEX submissions_user_id_created_at_idx ON submissions (user_id, created_at DESC);
CREATE INDEX submissions_problem_id_created_at_idx ON submissions (problem_id, created_at DESC);
CREATE INDEX submission_results_submission_id_idx ON submission_results (submission_id);
CREATE INDEX code_runs_user_id_created_at_idx ON code_runs (user_id, created_at DESC);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_active_token_hash_idx ON sessions (token_hash) WHERE revoked_at IS NULL;
CREATE INDEX group_members_user_id_group_id_idx ON group_members (user_id, group_id);
