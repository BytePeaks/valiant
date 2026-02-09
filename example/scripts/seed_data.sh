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
  local additional_metadata_json=$6 # New parameter, e.g. "{ \"key\": \"value\" }"

  if [ -z "$time_offset" ]; then
      local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  else
      # BSD/GNU date compatibility is tricky, assuming GNU date for linux container
      local timestamp=$(date -u -d "$time_offset" +"%Y-%m-%dT%H:%M:%SZ")
  fi

  local id=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 12 | head -n 1)
  
  # Randomly pick an environment
  local envs=("default" "payment-app")
  local env=${envs[$((RANDOM % ${#envs[@]}))]}

  local metadata_fields="\"author\": \"konrad\", \"env\": \"$env\", \"version\": \"v2.4.$((RANDOM % 10))\""

  if [ -n "$additional_metadata_json" ]; then
    # Remove leading/trailing braces from additional_metadata_json and append
    local cleaned_additional=$(echo $additional_metadata_json | sed 's/^{//;s/}$//')
    metadata_fields="$metadata_fields, $cleaned_additional"
  fi

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
      \"metadata\": { $metadata_fields }
    }"
  echo " Sent: $summary (env: $env)"
}

# Generate 10 events
send_event "Deployment of payment-service v2.4.0" "GitOps" "deployment_rollout" '["payment-service"]' "5 minutes ago" "{ \"git_commit_sha\": \"$(head /dev/urandom | tr -dc A-F0-9 | fold -w 7 | head -n 1)\", \"repository_url\": \"https://github.com/valiant-io/valiant\" }"
send_event "CI Pipeline #452: Merge & Test" "CI" "pipeline_success" '["payment-service", "order-service"]' "15 minutes ago" "{ \"jenkins_url\": \"http://jenkins.example.com\", \"jenkins_job_name\": \"valiant-ci\", \"jenkins_build_id\": \"$((RANDOM % 900 + 100))\" }"
send_event "Updated configmap payment-config" "GitOps" "configmap_update" '["payment-service"]' "45 minutes ago"
send_event "Deployment of inventory-service v1.1.0" "GitOps" "deployment_rollout" '["inventory-service"]' "2 hours ago"
send_event "Database schema migration (users)" "CI" "migration" '["payment-service", "user-service"]' "3 hours ago"
send_event "Canary Release: payment-service v2.5.0-rc1" "GitOps" "canary_start" '["payment-service"]' "4 hours ago"
send_event "Deployment of order-service v3.0.1" "GitOps" "deployment_rollout" '["order-service"]' "1 day ago" "{ \"argocd_url\": \"http://argocd.example.com\", \"argocd_app_name\": \"order-service-prod\" }"
send_event "CI Pipeline #440: Nightly Build" "CI" "pipeline_success" '["payment-service", "order-service", "inventory-service"]' "1 day 2 hours ago"
send_event "Infrastructure scale-up (node pool)" "GitOps" "infra_scale" '["cluster-nodes"]' "3 days ago"
send_event "Canary Release: order-service v3.1.0-beta" "GitOps" "canary_start" '["order-service"]' "4 days ago"

# --- New Deep Linking Test Events ---

# Proper GitHub Link
send_event "Feature Branch Merge: user-profile" "GitOps" "deployment_rollout" '["user-service"]' "60 minutes ago" "{ \"git_commit_sha\": \"$(head /dev/urandom | tr -dc A-F0-9 | fold -w 7 | head -n 1)\", \"repository_url\": \"https://github.com/valiant-io/valiant\" }"

# Broken GitHub Link (missing git_commit_sha)
send_event "Documentation Update" "Manual" "docs_change" '["docs-service"]' "90 minutes ago" "{ \"repository_url\": \"https://github.com/valiant-io/valiant\" }"

# Proper Jenkins Link
send_event "Jenkins Build: frontend-deploy" "CI" "pipeline_success" '["frontend-service"]' "150 minutes ago" "{ \"jenkins_url\": \"http://jenkins.example.com\", \"jenkins_job_name\": \"frontend-build\", \"jenkins_build_id\": \"$((RANDOM % 9000 + 1000))\" }"

# Broken Jenkins Link (missing jenkins_job_name from MetadataHas)
send_event "Jenkins Build: failed-test-run" "CI" "pipeline_failure" '["test-runner"]' "180 minutes ago" "{ \"jenkins_url\": \"http://jenkins.example.com\", \"jenkins_build_id\": \"$((RANDOM % 9000 + 1000))\" }"

# Proper ArgoCD Link
send_event "ArgoCD Sync: backend-app" "GitOps" "sync_successful" '["backend-service"]' "210 minutes ago" "{ \"argocd_url\": \"http://argocd.example.com\", \"argocd_app_name\": \"backend-prod\" }"

# Broken ArgoCD Link (missing argocd_app_name from MetadataHas)
send_event "ArgoCD Sync: config-update" "GitOps" "config_sync" '["config-service"]' "240 minutes ago" "{ \"argocd_url\": \"http://argocd.example.com\" }"

# Mixed Metadata - some valid links, some broken (missing MetadataHas keys), some broken (missing URLTemplate keys)
send_event "Mixed Event Testing" "Manual" "mixed_test" '["multi-service"]' "300 minutes ago" "{ \"git_commit_sha\": \"$(head /dev/urandom | tr -dc A-F0-9 | fold -w 7 | head -n 1)\", \"repository_url\": \"https://github.com/valiant-io/mixed-test\", \"jenkins_url\": \"http://jenkins.example.com/mixed-project\", \"argocd_url\": \"http://argocd.example.com/mixed-cluster\", \"template_missing_key_test_base\": \"http://test.com/\" }"

echo "Done seeding."

echo "Done seeding."

