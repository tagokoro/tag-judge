#!/bin/sh
set -eu

REGION="${AWS_DEFAULT_REGION:-ap-northeast-1}"
BUCKET_NAME="${JUDGE_STORAGE_BUCKET:-judge-storage}"

awslocal s3api create-bucket \
  --region "$REGION" \
  --bucket "$BUCKET_NAME" \
  --create-bucket-configuration "LocationConstraint=$REGION"
