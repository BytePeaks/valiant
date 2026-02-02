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
        [int]$MinutesAgo = 0
    )

    $timestamp = (Get-Date).AddMinutes(-$MinutesAgo).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    # Generate a random 12-char ID
    $id = -join ((48..57) + (65..90) + (97..122) | Get-Random -Count 12 | ForEach-Object { [char]$_ })

    # Randomly pick an environment
    $envs = @("default", "payment-app")
    $env = $envs | Get-Random

    $body = @{
        id = $id
        trigger_type = $Trigger
        execution_id = "exec-$id"
        change_type = $Type
        timestamp = $timestamp
        affected_services = $Services
        summary = $Summary
        metadata = @{
            author = "konrad"
            env = $env
            version = "v2.4.$(Get-Random -Minimum 0 -Maximum 9)"
        }
    } | ConvertTo-Json -Depth 5

    try {
        $response = Invoke-RestMethod -Uri "$ApiUrl/events" -Method Post -Body $body -ContentType "application/json"
        Write-Host " Sent: $Summary ($MinutesAgo min ago, env: $env)"
    } catch {
        Write-Error "Failed to send event '$Summary': $_"
    }
}

Send-Event -Summary "Deployment of payment-service v2.4.0" -Trigger "GitOps" -Type "deployment_rollout" -Services @("payment-service") -MinutesAgo 5
Send-Event -Summary "CI Pipeline #452: Merge & Test" -Trigger "CI" -Type "pipeline_success" -Services @("payment-service", "order-service") -MinutesAgo 15
Send-Event -Summary "Updated configmap payment-config" -Trigger "GitOps" -Type "configmap_update" -Services @("payment-service") -MinutesAgo 45
Send-Event -Summary "Deployment of inventory-service v1.1.0" -Trigger "GitOps" -Type "deployment_rollout" -Services @("inventory-service") -MinutesAgo 120
Send-Event -Summary "Database schema migration (users)" -Trigger "CI" -Type "migration" -Services @("payment-service", "user-service") -MinutesAgo 180
Send-Event -Summary "Canary Release: payment-service v2.5.0-rc1" -Trigger "GitOps" -Type "canary_start" -Services @("payment-service") -MinutesAgo 240
Send-Event -Summary "Deployment of order-service v3.0.1" -Trigger "GitOps" -Type "deployment_rollout" -Services @("order-service") -MinutesAgo 1440
Send-Event -Summary "CI Pipeline #440: Nightly Build" -Trigger "CI" -Type "pipeline_success" -Services @("payment-service", "order-service", "inventory-service") -MinutesAgo 1560
Send-Event -Summary "Infrastructure scale-up (node pool)" -Trigger "GitOps" -Type "infra_scale" -Services @("cluster-nodes") -MinutesAgo 4320
Send-Event -Summary "Canary Release: order-service v3.1.0-beta" -Trigger "GitOps" -Type "canary_start" -Services @("order-service") -MinutesAgo 5760

Write-Host "Done seeding."
