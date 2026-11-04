#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <api-base-url> <existing-user-uuid>" >&2
  exit 2
fi

api_base_url="${1%/}"
user_id="$2"
problem_id=""

json_field() {
  python3 -c 'import json,sys; value=json.load(sys.stdin); print(value["data"][sys.argv[1]])' "$1"
}

api_request() {
  method="$1"
  path="$2"
  body="${3:-}"
  args=(
    --fail-with-body --silent --show-error
    --request "$method"
    --header "X-User-ID: $user_id"
    --header "X-User-Role: admin"
  )
  if [ -n "$body" ]; then
    args+=(--header 'Content-Type: application/json' --data "$body")
  fi
  curl "${args[@]}" "$api_base_url$path"
}

cleanup() {
  if [ -n "$problem_id" ]; then
    api_request PATCH "/api/v1/problems/$problem_id" '{"visibility":"private"}' >/dev/null || true
  fi
}
trap cleanup EXIT

curl --fail-with-body --silent --show-error "$api_base_url/healthz" >/dev/null

problem_response="$(api_request POST /api/v1/problems '{
  "title":"AWS smoke test",
  "statement":"Echo one line",
  "time_limit_ms":1000,
  "memory_limit_kb":65536,
  "visibility":"private"
}')"
problem_id="$(printf '%s' "$problem_response" | json_field id)"

api_request POST "/api/v1/problems/$problem_id/test-cases" '{
  "input":"hello\\n",
  "expected_output":"hello\\n",
  "is_sample":false
}' >/dev/null

api_request PATCH "/api/v1/problems/$problem_id" '{"visibility":"public"}' >/dev/null

submission_response="$(api_request POST "/api/v1/problems/$problem_id/submissions" '{
  "language":"python",
  "source_code":"print(input())"
}')"
submission_id="$(printf '%s' "$submission_response" | json_field id)"

for _ in $(seq 1 60); do
  result="$(api_request GET "/api/v1/submissions/$submission_id")"
  status="$(printf '%s' "$result" | json_field status)"
  case "$status" in
    pending|running)
      sleep 2
      ;;
    ac)
      printf '%s\n' "$result"
      echo "AWS submission smoke test passed: submission $submission_id is AC" >&2
      exit 0
      ;;
    *)
      printf '%s\n' "$result" >&2
      echo "AWS submission smoke test failed: submission $submission_id is $status" >&2
      exit 1
      ;;
  esac
done

echo "AWS submission smoke test timed out: submission $submission_id did not finish" >&2
exit 1
