#!/bin/sh
set -eu

REGION="${AWS_DEFAULT_REGION:-ap-northeast-1}"
QUEUE_NAME="${JUDGE_SUBMISSIONS_QUEUE_NAME:-judge-submissions}"
DLQ_NAME="${JUDGE_SUBMISSIONS_DLQ_NAME:-judge-submissions-dlq}"

awslocal sqs create-queue \
  --region "$REGION" \
  --queue-name "$DLQ_NAME"

DLQ_URL="$(awslocal sqs get-queue-url \
  --region "$REGION" \
  --queue-name "$DLQ_NAME" \
  --query 'QueueUrl' \
  --output text)"

DLQ_ARN="$(awslocal sqs get-queue-attributes \
  --region "$REGION" \
  --queue-url "$DLQ_URL" \
  --attribute-names QueueArn \
  --query 'Attributes.QueueArn' \
  --output text)"

REDRIVE_POLICY="{\"deadLetterTargetArn\":\"$DLQ_ARN\",\"maxReceiveCount\":\"3\"}"
REDRIVE_POLICY_JSON_VALUE="$(printf '%s' "$REDRIVE_POLICY" | sed 's/\\/\\\\/g; s/"/\\"/g')"
ATTRIBUTES_FILE="/tmp/judge-submissions-attributes.json"

cat > "$ATTRIBUTES_FILE" <<EOF
{
  "VisibilityTimeout": "30",
  "RedrivePolicy": "$REDRIVE_POLICY_JSON_VALUE"
}
EOF

awslocal sqs create-queue \
  --region "$REGION" \
  --queue-name "$QUEUE_NAME" \
  --attributes "file://$ATTRIBUTES_FILE"
