#!/bin/bash

API_URL=${1:-"http://localhost:8080/api/v1"}

echo "Seeding Valiant with mock data at $API_URL..."

# Helper to generate a random 40-char SHA
generate_sha() {
  head /dev/urandom | tr -dc a-f0-9 | head -c 40
}

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
  local envs=("default" "payment-app" "inventory-app" "order-app")
  local env=${envs[$((RANDOM % ${#envs[@]}))]}

  local metadata_fields="\"author\": \"konrad\", \"env\": \"$env\", \"version\": \"v2.4.$((RANDOM % 10))\""

  if [ -n "$additional_metadata_json" ]; then
    # Remove leading/trailing braces from additional_metadata_json and append
    local cleaned_additional=$(echo "$additional_metadata_json" | sed 's/^{//;s/}$//')
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
  echo " Sent: $summary (env: $env, trigger: $trigger, type: $type)"
}

echo "--- Generating Core Events ---"

# Generate a CI Intent event with a specific SHA
PAYMENT_SERVICE_SHA_1=$(generate_sha)
send_event "CI Pipeline #452: payment-service new build" "CI" "build_success" '["payment-service", "order-service"]' "15 minutes ago" "{ \"git_commit_sha\": \"$PAYMENT_SERVICE_SHA_1\", \"image_tag\": \"my-registry/payment-service:$PAYMENT_SERVICE_SHA_1\", \"jenkins_url\": \"http://jenkins.example.com\", \"jenkins_job_name\": \"valiant-ci\", \"jenkins_build_id\": \"$((RANDOM % 900 + 100))\" }"

# Correlated ArgoCD Execution event for payment-service
send_event "Deployment of payment-service v2.4.0 via ArgoCD" "argocd" "deployment_rollout" '["payment-service"]' "5 minutes ago" "{ \"git_commit_sha\": \"$PAYMENT_SERVICE_SHA_1\", \"image_tag\": \"my-registry/payment-service:$PAYMENT_SERVICE_SHA_1\", \"argocd_url\": \"http://argocd.example.com\", \"argocd_app_name\": \"payment-service-prod\" }"

# Another CI Intent event for inventory-service
INVENTORY_SERVICE_SHA_1=$(generate_sha)
send_event "CI Pipeline #453: inventory-service new build" "github-actions" "build_success" '["inventory-service"]' "2 hours 15 minutes ago" "{ \"git_commit_sha\": \"$INVENTORY_SERVICE_SHA_1\", \"image_tag\": \"my-registry/inventory-service:$INVENTORY_SERVICE_SHA_1\", \"github_run_url\": \"https://github.com/valiant-io/valiant/actions/runs/$((RANDOM % 100000 + 10000))\" }"

# Correlated Kubernetes-API Execution for inventory-service (simulating kubectl apply)
send_event "Deployment of inventory-service v1.1.0 (kubectl apply)" "kubernetes-api" "deployment_rollout" '["inventory-service"]' "2 hours ago" "{ \"git_commit_sha\": \"$INVENTORY_SERVICE_SHA_1\", \"image_tag\": \"my-registry/inventory-service:$INVENTORY_SERVICE_SHA_1\" }"

# ConfigMap update event (Execution)
CONFIG_SHA=$(generate_sha)
send_event "Updated configmap payment-config" "gitops-config" "configmap_update" '["payment-service"]' "45 minutes ago" "{ \"git_commit_sha\": \"$CONFIG_SHA\", \"config_name\": \"payment-config\" }"

# Manual database migration (Intent/Execution - depends on API config)
send_event "Database schema migration (users)" "manual" "migration" '["payment-service", "user-service"]' "3 hours ago"

# Uncorrelated Deployment
send_event "Deployment of order-service v3.0.1 (unknown source)" "kubernetes-api" "deployment_rollout" '["order-service"]' "1 day ago" "{ \"image_tag\": \"my-registry/order-service:v3.0.1\" }"

# Old CI Event (Intent)
send_event "CI Pipeline #440: Nightly Build" "CI" "pipeline_success" '["payment-service", "order-service", "inventory-service"]' "1 day 2 hours ago" "{ \"git_commit_sha\": \"$(generate_sha)\" }"

echo "--- Generating Deep Linking Test Events ---"

# Proper GitHub Link (Intent + Execution pair for user-service)
GITHUB_SHA=$(generate_sha)

# CI Intent for user-service (sent first so it exists when execution is POSTed)
send_event "CI Pipeline #460: user-service build" "github-actions" "build_success" '["user-service"]' "70 minutes ago" "{ \"git_commit_sha\": \"$GITHUB_SHA\", \"image_tag\": \"my-registry/user-service:$GITHUB_SHA\", \"github_run_url\": \"https://github.com/valiant-io/valiant/actions/runs/$((RANDOM % 100000 + 10000))\", \"repository_url\": \"https://github.com/valiant-io/valiant\" }"

# Correlated Execution for user-service
send_event "Feature Branch Merge: user-profile rollout" "argocd" "deployment_rollout" '["user-service"]' "60 minutes ago" "{ \"git_commit_sha\": \"$GITHUB_SHA\", \"repository_url\": \"https://github.com/valiant-io/valiant\", \"image_tag\": \"my-registry/user-service:$GITHUB_SHA\" }"

# Broken GitHub Link (missing git_commit_sha but has repo URL - Intent)
send_event "Documentation Update build" "CI" "docs_build" '["docs-service"]' "90 minutes ago" "{ \"repository_url\": \"https://github.com/valiant-io/valiant\" }"

# Proper Jenkins Link (Intent)
JENKINS_SHA=$(generate_sha)
send_event "Jenkins Build: frontend-deploy" "jenkins" "build_success" '["frontend-service"]' "150 minutes ago" "{ \"git_commit_sha\": \"$JENKINS_SHA\", \"jenkins_url\": \"http://jenkins.example.com\", \"jenkins_job_name\": \"frontend-build\", \"jenkins_build_id\": \"$((RANDOM % 9000 + 1000))\" }"

# Broken Jenkins Link (missing jenkins_job_name from MetadataHas - Intent)
send_event "Jenkins Build: failed-test-run" "jenkins" "pipeline_failure" '["test-runner"]' "180 minutes ago" "{ \"jenkins_url\": \"http://jenkins.example.com\", \"jenkins_build_id\": \"$((RANDOM % 9000 + 1000))\" }"

# Proper ArgoCD Link (Intent + Execution pair for backend-service)
ARGO_SHA=$(generate_sha)

# CI Intent for backend-service (sent first)
send_event "CI Pipeline #470: backend-service build" "CI" "build_success" '["backend-service"]' "220 minutes ago" "{ \"git_commit_sha\": \"$ARGO_SHA\", \"image_tag\": \"my-registry/backend-service:$ARGO_SHA\" }"

# Correlated ArgoCD Execution for backend-service
send_event "ArgoCD Sync: backend-app deployment" "argocd" "deployment_rollout" '["backend-service"]' "210 minutes ago" "{ \"git_commit_sha\": \"$ARGO_SHA\", \"argocd_url\": \"http://argocd.example.com\", \"argocd_app_name\": \"backend-prod\", \"image_tag\": \"my-registry/backend-service:$ARGO_SHA\" }"

# Broken ArgoCD Link (missing argocd_app_name from MetadataHas - Execution)
send_event "ArgoCD Sync: config-update" "argocd" "config_sync" '["config-service"]' "240 minutes ago" "{ \"argocd_url\": \"http://argocd.example.com\" }"

# Mixed Metadata Event (Manual, can be Intent or Execution based on API)
MIXED_SHA=$(generate_sha)
send_event "Mixed Event Testing" "manual" "mixed_test" '["multi-service"]' "300 minutes ago" "{ \"git_commit_sha\": \"$MIXED_SHA\", \"repository_url\": \"https://github.com/valiant-io/mixed-test\", \"jenkins_url\": \"http://jenkins.example.com/mixed-project\", \"argocd_url\": \"http://argocd.example.com/mixed-cluster\", \"template_missing_key_test_base\": \"http://test.com/\" }"

echo "Done seeding."