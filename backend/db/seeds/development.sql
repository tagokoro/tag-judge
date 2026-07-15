INSERT INTO users (id, provider, provider_user_id, name, email, role)
VALUES
    ('00000000-0000-4000-8000-000000000001', 'google', 'seed-author', 'Seed Author', 'author@example.com', 'author'),
    ('00000000-0000-4000-8000-000000000002', 'google', 'seed-admin', 'Seed Admin', 'admin@example.com', 'admin')
ON CONFLICT (provider, provider_user_id) DO UPDATE
SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    role = EXCLUDED.role,
    updated_at = now();

INSERT INTO problems (
    id,
    title,
    statement,
    time_limit_ms,
    memory_limit_kb,
    visibility,
    created_by
)
VALUES
    (
        1,
        'A + B',
        $md$# A + B

整数 $A$ と $B$ が与えられます。
$A + B$ を出力してください。

## 制約

- $0 \le A, B \le 100$
- 入力はすべて整数

## 入力

```text
A B
```

## 出力

```text
A + B
```

## 入力例 1

```text
1 2
```

## 出力例 1

```text
3
```
$md$,
        2000,
        262144,
        'public',
        '00000000-0000-4000-8000-000000000001'
    ),
    (
        2,
        'Private Echo',
        $md$# Private Echo

この問題は private problem の疎通確認用です。
公開問題一覧には表示されません。

文字列 $S$ が与えられます。
$S$ をそのまま出力してください。

## 入力

```text
S
```

## 出力

```text
S
```
$md$,
        1000,
        262144,
        'private',
        '00000000-0000-4000-8000-000000000001'
    )
ON CONFLICT (id) DO UPDATE
SET
    title = EXCLUDED.title,
    statement = EXCLUDED.statement,
    time_limit_ms = EXCLUDED.time_limit_ms,
    memory_limit_kb = EXCLUDED.memory_limit_kb,
    visibility = EXCLUDED.visibility,
    created_by = EXCLUDED.created_by,
    updated_at = now();

INSERT INTO test_cases (id, problem_id, input, expected_output, is_sample, score)
VALUES
    (1, 1, '1 2
', '3
', true, 1),
    (2, 1, '10 20
', '30
', false, 1),
    (3, 2, 'hello
', 'hello
', true, 1),
    (4, 2, 'tag-judge
', 'tag-judge
', false, 1)
ON CONFLICT (id) DO UPDATE
SET
    problem_id = EXCLUDED.problem_id,
    input = EXCLUDED.input,
    expected_output = EXCLUDED.expected_output,
    is_sample = EXCLUDED.is_sample,
    score = EXCLUDED.score;

SELECT setval('problems_id_seq', (SELECT max(id) FROM problems));
SELECT setval('test_cases_id_seq', (SELECT max(id) FROM test_cases));
