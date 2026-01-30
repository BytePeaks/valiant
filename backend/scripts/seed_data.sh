#!/bin/bash

API_URL=${1:-"http://localhost:8080/api/v1"}

echo "Seeding Valiant with mock data at $API_URL..."

# Helper to send event
send_event() {
  local summary=$1
  local trigger=$2
  local type=$3
  local services_json=$4 # e.g. '["service-a"]'
  local time_offset=$5 # e.g. "10 minutes ago"
  
  if [ -z "$time_offset" ]; then
      local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  else
      # BSD/GNU date compatibility is tricky, assuming GNU date for linux container
      local timestamp=$(date -u -d "$time_offset" +"%Y-%m-%dT%H:%M:%SZ")
  fi

  local id=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 12 | head -n 1)

  curl -s -X POST "$API_URL/events" \
    -H "Content-Type: application/json" \
    -d "{
      \"id\": \"$id\",
      \"trigger_type\": \"$trigger\",
      \"execution_id\": \"exec-$id\",
      \"change_type\": \"$type\",
      \"timestamp\": \"$timestamp\",
      \"affected_services\": $services_json,
      \"summary\": \"$summary\",
      \"metadata\": {
        \"author\": \"konrad\",
        \"env\": \"production\",
        \"version\": \"v2.4.$((RANDOM % 10))\"
      }
    }"
  echo " Sent: $summary"
}

# Generate 12 events
send_event "Deployment of payment-service v2.4.0" "GitOps" "deployment_rollout" '["payment-service"]' "5 minutes ago"
send_event "CI Pipeline #452: Merge & Test" "CI" "pipeline_success" '["payment-service", "order-service"]' "15 minutes ago"
send_event "Updated configmap payment-config" "GitOps" "configmap_update" '["payment-service"]' "45 minutes ago"
send_event "Manual rollback of order-service" "manual" "rollback" '["order-service"]' "1 hour ago"
send_event "Deployment of inventory-service v1.1.0" "GitOps" "deployment_rollout" '["inventory-service"]' "2 hours ago"
send_event "Database schema migration (users)" "CI" "migration" '["payment-service", "user-service"]' "3 hours ago"
send_event "Canary Release: payment-service v2.5.0-rc1" "GitOps" "canary_start" '["payment-service"]' "4 hours ago"
send_event "Manual cache flush (redis)" "manual" "ops_action" '["inventory-service"]' "5 hours ago"
send_event "Deployment of order-service v3.0.1" "GitOps" "deployment_rollout" '["order-service"]' "1 day ago"
send_event "CI Pipeline #440: Nightly Build" "CI" "pipeline_success" '["payment-service", "order-service", "inventory-service"]' "1 day 2 hours ago"
send_event "Hotfix: payment-service gateway timeout" "manual" "hotfix" '["payment-service"]' "2 days ago"
send_event "Infrastructure scale-up (node pool)" "GitOps" "infra_scale" '["cluster-nodes"]' "3 days ago"

echo "Done seeding."

echo "Done seeding."

