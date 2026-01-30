#!/bin/bash

API_URL=${1:-"http://localhost:8080/api/v1"}

echo "Seeding Valiant with mock data at $API_URL..."

# Helper to send event
send_event() {
  local summary=$1
  local source=$2
  local type=$3
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  local id=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 12 | head -n 1)

  curl -s -X POST "$API_URL/events" \
    -H "Content-Type: application/json" \
    -d "{
      \"id\": \"$id\",
      \"source\": \"$source\",
      \"change_type\": \"$type\",
      \"timestamp\": \"$timestamp\",
      \"affected_services\": [\"payment-service\", \"order-service\"],
      \"summary\": \"$summary\",
      \"metadata\": {
        \"author\": \"konrad\",
        \"env\": \"production\"
      }
    }"
  echo " Sent: $summary"
}

send_event "Deployment of payment-service v1.2.3" "kubernetes" "deployment_rollout"
send_event "Updated configmap payment-config" "kubernetes" "configmap_update"
send_event "Merged PR #452: Update order processing logic" "git" "pr_merge"
send_event "CI Build #892 succeeded" "ci-cd" "build_success"

echo "Done seeding."

