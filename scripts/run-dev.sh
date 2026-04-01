#!/usr/bin/env bash

set -euo pipefail

# if [[ ! -f ".env.local" ]]; then
#   echo ".env.local is required"
#   echo "Create it in the repo root, then run this script again."
#   exit 1
# fi

set -a
source ~/.secrets/.env.local
set +a

exec go run ./cmd/api
