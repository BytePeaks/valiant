param (
    [string]$ApiUrl = "http://localhost:8080/api/v1"
)

Write-Host "Seeding Valiant with mock data at $ApiUrl..."

# Helper to generate a random 40-char hex SHA
function New-Sha {
    $bytes = New-Object byte[] 20
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    return ($bytes | ForEach-Object { $_.ToString("x2") }) -join ''
}

function Send-Event {
    param (
        [string]$Summary,
        [string]$Trigger,
        [string]$Type,
        [string[]]$Services,
        [int]$MinutesAgo = 0,
        [hashtable]$MetadataArgs = @{}
    )

    $timestamp = (Get-Date).AddMinutes(-$MinutesAgo).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    # Generate a random 12-char ID
    $id = -join ((48..57) + (65..90) + (97..122) | Get-Random -Count 12 | ForEach-Object { [char]$_ })

    # Randomly pick an environment
    $envs = @("default", "payment-app", "inventory-app", "order-app")
    $env = $envs | Get-Random

    $baseMetadata = @{
        author = "konrad"
        env = $env
        version = "v2.4.$(Get-Random -Minimum 0 -Maximum 9)"
    }
    # Merge MetadataArgs into baseMetadata
    $MetadataArgs.GetEnumerator() | ForEach-Object { $baseMetadata[$_.Key] = $_.Value }

    $body = @{
        id = $id
        trigger_type = $Trigger
        execution_id = "exec-$id"
        change_type = $Type
        timestamp = $timestamp
        affected_services = $Services
        summary = $Summary
        metadata = $baseMetadata
    } | ConvertTo-Json -Depth 5

    try {
        $response = Invoke-RestMethod -Uri "$ApiUrl/events" -Method Post -Body $body -ContentType "application/json"
        Write-Host " Sent: $Summary (env: $env, trigger: $Trigger, type: $Type)"
    } catch {
        Write-Error "Failed to send event '$Summary': $_"
    }
}

Write-Host "--- Generating Core Events ---"

# Generate a CI Intent event with a specific SHA
$PaymentSha1 = New-Sha
Send-Event -Summary "CI Pipeline #452: payment-service new build" -Trigger "CI" -Type "build_success" -Services @("payment-service", "order-service") -MinutesAgo 15 -MetadataArgs @{
    git_commit_sha = $PaymentSha1
    image_tag = "my-registry/payment-service:$PaymentSha1"
    jenkins_url = "http://jenkins.example.com"
    jenkins_job_name = "valiant-ci"
    jenkins_build_id = [string](Get-Random -Minimum 100 -Maximum 999)
}

# Correlated ArgoCD Execution event for payment-service
Send-Event -Summary "Deployment of payment-service v2.4.0 via ArgoCD" -Trigger "argocd" -Type "deployment_rollout" -Services @("payment-service") -MinutesAgo 5 -MetadataArgs @{
    git_commit_sha = $PaymentSha1
    image_tag = "my-registry/payment-service:$PaymentSha1"
    argocd_url = "http://argocd.example.com"
    argocd_app_name = "payment-service-prod"
}

# Another CI Intent event for inventory-service
$InventorySha1 = New-Sha
Send-Event -Summary "CI Pipeline #453: inventory-service new build" -Trigger "github-actions" -Type "build_success" -Services @("inventory-service") -MinutesAgo 135 -MetadataArgs @{
    git_commit_sha = $InventorySha1
    image_tag = "my-registry/inventory-service:$InventorySha1"
    github_run_url = "https://github.com/valiant-io/valiant/actions/runs/$(Get-Random -Minimum 10000 -Maximum 99999)"
}

# Correlated Kubernetes-API Execution for inventory-service (simulating kubectl apply)
Send-Event -Summary "Deployment of inventory-service v1.1.0 (kubectl apply)" -Trigger "kubernetes-api" -Type "deployment_rollout" -Services @("inventory-service") -MinutesAgo 120 -MetadataArgs @{
    git_commit_sha = $InventorySha1
    image_tag = "my-registry/inventory-service:$InventorySha1"
}

# ConfigMap update event (Execution)
$ConfigSha = New-Sha
Send-Event -Summary "Updated configmap payment-config" -Trigger "gitops-config" -Type "configmap_update" -Services @("payment-service") -MinutesAgo 45 -MetadataArgs @{
    git_commit_sha = $ConfigSha
    config_name = "payment-config"
}

# Manual database migration (Intent/Execution - depends on API config)
Send-Event -Summary "Database schema migration (users)" -Trigger "manual" -Type "migration" -Services @("payment-service", "user-service") -MinutesAgo 180

# Uncorrelated Deployment (orphaned execution - no matching CI event)
Send-Event -Summary "Deployment of order-service v3.0.1 (unknown source)" -Trigger "kubernetes-api" -Type "deployment_rollout" -Services @("order-service") -MinutesAgo 1440 -MetadataArgs @{
    image_tag = "my-registry/order-service:v3.0.1"
}

# Old CI Event (Intent)
Send-Event -Summary "CI Pipeline #440: Nightly Build" -Trigger "CI" -Type "pipeline_success" -Services @("payment-service", "order-service", "inventory-service") -MinutesAgo 1560 -MetadataArgs @{
    git_commit_sha = (New-Sha)
}

Write-Host "--- Generating Deep Linking Test Events ---"

# Proper GitHub Link (Execution with correlated CI Intent)
$GithubSha = New-Sha

# CI Intent for user-service
Send-Event -Summary "CI Pipeline #460: user-service build" -Trigger "github-actions" -Type "build_success" -Services @("user-service") -MinutesAgo 70 -MetadataArgs @{
    git_commit_sha = $GithubSha
    image_tag = "my-registry/user-service:$GithubSha"
    github_run_url = "https://github.com/valiant-io/valiant/actions/runs/$(Get-Random -Minimum 10000 -Maximum 99999)"
    repository_url = "https://github.com/valiant-io/valiant"
}

# Correlated Execution for user-service
Send-Event -Summary "Feature Branch Merge: user-profile rollout" -Trigger "argocd" -Type "deployment_rollout" -Services @("user-service") -MinutesAgo 60 -MetadataArgs @{
    git_commit_sha = $GithubSha
    image_tag = "my-registry/user-service:$GithubSha"
    repository_url = "https://github.com/valiant-io/valiant"
}

# Broken GitHub Link (missing git_commit_sha but has repo URL - Intent)
Send-Event -Summary "Documentation Update build" -Trigger "CI" -Type "docs_build" -Services @("docs-service") -MinutesAgo 90 -MetadataArgs @{
    repository_url = "https://github.com/valiant-io/valiant"
}

# Proper Jenkins Link (Intent)
$JenkinsSha = New-Sha
Send-Event -Summary "Jenkins Build: frontend-deploy" -Trigger "jenkins" -Type "build_success" -Services @("frontend-service") -MinutesAgo 150 -MetadataArgs @{
    git_commit_sha = $JenkinsSha
    jenkins_url = "http://jenkins.example.com"
    jenkins_job_name = "frontend-build"
    jenkins_build_id = [string](Get-Random -Minimum 1000 -Maximum 9999)
}

# Broken Jenkins Link (missing jenkins_job_name from MetadataHas - Intent)
Send-Event -Summary "Jenkins Build: failed-test-run" -Trigger "jenkins" -Type "pipeline_failure" -Services @("test-runner") -MinutesAgo 180 -MetadataArgs @{
    jenkins_url = "http://jenkins.example.com"
    jenkins_build_id = [string](Get-Random -Minimum 1000 -Maximum 9999)
}

# Proper ArgoCD Link (Execution with correlated CI Intent)
$ArgoSha = New-Sha

# CI Intent for backend-service
Send-Event -Summary "CI Pipeline #470: backend-service build" -Trigger "CI" -Type "build_success" -Services @("backend-service") -MinutesAgo 220 -MetadataArgs @{
    git_commit_sha = $ArgoSha
    image_tag = "my-registry/backend-service:$ArgoSha"
}

# Correlated ArgoCD Execution for backend-service
Send-Event -Summary "ArgoCD Sync: backend-app deployment" -Trigger "argocd" -Type "deployment_rollout" -Services @("backend-service") -MinutesAgo 210 -MetadataArgs @{
    git_commit_sha = $ArgoSha
    image_tag = "my-registry/backend-service:$ArgoSha"
    argocd_url = "http://argocd.example.com"
    argocd_app_name = "backend-prod"
}

# Broken ArgoCD Link (missing argocd_app_name from MetadataHas - Execution)
Send-Event -Summary "ArgoCD Sync: config-update" -Trigger "argocd" -Type "config_sync" -Services @("config-service") -MinutesAgo 240 -MetadataArgs @{
    argocd_url = "http://argocd.example.com"
}

# Mixed Metadata Event (Manual, can be Intent or Execution based on API)
$MixedSha = New-Sha
Send-Event -Summary "Mixed Event Testing" -Trigger "manual" -Type "mixed_test" -Services @("multi-service") -MinutesAgo 300 -MetadataArgs @{
    git_commit_sha = $MixedSha
    repository_url = "https://github.com/valiant-io/mixed-test"
    jenkins_url = "http://jenkins.example.com/mixed-project"
    argocd_url = "http://argocd.example.com/mixed-cluster"
    template_missing_key_test_base = "http://test.com/"
}

Write-Host "Done seeding."
