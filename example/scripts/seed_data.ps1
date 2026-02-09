param (
    [string]$ApiUrl = "http://localhost:8080/api/v1"
)

Write-Host "Seeding Valiant with mock data at $ApiUrl..."

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
    $envs = @("default", "payment-app")
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
        Write-Host " Sent: $Summary ($MinutesAgo min ago, env: $env)"
    } catch {
        Write-Error "Failed to send event '$Summary': $_"
    }
}

Send-Event -Summary "Deployment of payment-service v2.4.0" -Trigger "GitOps" -Type "deployment_rollout" -Services @("payment-service") -MinutesAgo 5 -MetadataArgs @{
    git_commit_sha = ("{0:x}" -f (Get-Random -Minimum 0 -Maximum 16777215)) # 6 hex chars
    repository_url = "https://github.com/valiant-io/valiant"
}
Send-Event -Summary "CI Pipeline #452: Merge & Test" -Trigger "CI" -Type "pipeline_success" -Services @("payment-service", "order-service") -MinutesAgo 15 -MetadataArgs @{
    jenkins_url = "http://jenkins.example.com"
    jenkins_job_name = "valiant-ci"
    jenkins_build_id = [string](Get-Random -Minimum 100 -Maximum 999)
}
Send-Event -Summary "Updated configmap payment-config" -Trigger "GitOps" -Type "configmap_update" -Services @("payment-service") -MinutesAgo 45
Send-Event -Summary "Deployment of inventory-service v1.1.0" -Trigger "GitOps" -Type "deployment_rollout" -Services @("inventory-service") -MinutesAgo 120
Send-Event -Summary "Database schema migration (users)" -Trigger "CI" -Type "migration" -Services @("payment-service", "user-service") -MinutesAgo 180
Send-Event -Summary "Canary Release: payment-service v2.5.0-rc1" -Trigger "GitOps" -Type "canary_start" -Services @("payment-service") -MinutesAgo 240
Send-Event -Summary "Deployment of order-service v3.0.1" -Trigger "GitOps" -Type "deployment_rollout" -Services @("order-service") -MinutesAgo 1440 -MetadataArgs @{
    argocd_url = "http://argocd.example.com"
    argocd_app_name = "order-service-prod"
}
Send-Event -Summary "CI Pipeline #440: Nightly Build" -Trigger "CI" -Type "pipeline_success" -Services @("payment-service", "order-service", "inventory-service") -MinutesAgo 1560
Send-Event -Summary "Infrastructure scale-up (node pool)" -Trigger "GitOps" -Type "infra_scale" -Services @("cluster-nodes") -MinutesAgo 4320
Send-Event -Summary "Canary Release: order-service v3.1.0-beta" -Trigger "GitOps" -Type "canary_start" -Services @("order-service") -MinutesAgo 5760

# --- New Deep Linking Test Events ---

# Proper GitHub Link
Send-Event -Summary "Feature Branch Merge: user-profile" -Trigger "GitOps" -Type "deployment_rollout" -Services @("user-service") -MinutesAgo 60 -MetadataArgs @{
    git_commit_sha = ("{0:x}" -f (Get-Random -Minimum 0 -Maximum 16777215)) # 6 hex chars
    repository_url = "https://github.com/valiant-io/valiant"
}

# Broken GitHub Link (missing git_commit_sha)
Send-Event -Summary "Documentation Update" -Trigger "Manual" -Type "docs_change" -Services @("docs-service") -MinutesAgo 90 -MetadataArgs @{
    repository_url = "https://github.com/valiant-io/valiant"
}

# Proper Jenkins Link
Send-Event -Summary "Jenkins Build: frontend-deploy" -Trigger "CI" -Type "pipeline_success" -Services @("frontend-service") -MinutesAgo 150 -MetadataArgs @{
    jenkins_url = "http://jenkins.example.com"
    jenkins_job_name = "frontend-build"
    jenkins_build_id = [string](Get-Random -Minimum 1000 -Maximum 9999)
}

# Broken Jenkins Link (missing jenkins_job_name from MetadataHas)
Send-Event -Summary "Jenkins Build: failed-test-run" -Trigger "CI" -Type "pipeline_failure" -Services @("test-runner") -MinutesAgo 180 -MetadataArgs @{
    jenkins_url = "http://jenkins.example.com"
    jenkins_build_id = [string](Get-Random -Minimum 1000 -Maximum 9999)
}

# Proper ArgoCD Link
Send-Event -Summary "ArgoCD Sync: backend-app" -Trigger "GitOps" -Type "sync_successful" -Services @("backend-service") -MinutesAgo 210 -MetadataArgs @{
    argocd_url = "http://argocd.example.com"
    argocd_app_name = "backend-prod"
}

# Broken ArgoCD Link (missing argocd_app_name from MetadataHas)
Send-Event -Summary "ArgoCD Sync: config-update" -Trigger "GitOps" -Type "config_sync" -Services @("config-service") -MinutesAgo 240 -MetadataArgs @{
    argocd_url = "http://argocd.example.com"
}

# Mixed Metadata - some valid links, some broken (missing MetadataHas keys), some broken (missing URLTemplate keys)
Send-Event -Summary "Mixed Event Testing" -Trigger "Manual" -Type "mixed_test" -Services @("multi-service") -MinutesAgo 300 -MetadataArgs @{
    # Valid for GitHub link
    git_commit_sha = ("{0:x}" -f (Get-Random -Minimum 0 -Maximum 16777215))
    repository_url = "https://github.com/valiant-io/mixed-test"

    # Incomplete for Jenkins link (missing jenkins_job_name and jenkins_build_id from MetadataHas)
    jenkins_url = "http://jenkins.example.com/mixed-project"

    # Incomplete for ArgoCD link (missing argocd_app_name from MetadataHas)
    argocd_url = "http://argocd.example.com/mixed-cluster"

    # Key used only in a hypothetical URLTemplate to test '<no value>' skipping (not actually added to config.yaml but good to test behavior)
    template_missing_key_test_base = "http://test.com/"
}

Write-Host "Done seeding."
