param (
    [string]$ApiUrl = "http://localhost:8080/api/v1"
)

Write-Host "Seeding Valiant with mock data at $ApiUrl..."

function Send-Event {
    param (
        [string]$Summary,
        [string]$Source,
        [string]$Type
    )

    $timestamp = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    # Generate a random 12-char ID
    $id = -join ((48..57) + (65..90) + (97..122) | Get-Random -Count 12 | ForEach-Object { [char]$_ })

    $body = @{
        id = $id
        source = $source
        change_type = $Type
        timestamp = $timestamp
        affected_services = @("payment-service", "order-service")
        summary = $Summary
        metadata = @{
            author = "konrad"
            env = "production"
        }
    } | ConvertTo-Json -Depth 5

    try {
        $response = Invoke-RestMethod -Uri "$ApiUrl/events" -Method Post -Body $body -ContentType "application/json"
        Write-Host " Sent: $Summary"
    } catch {
        Write-Error "Failed to send event '$Summary': $_"
    }
}

Send-Event -Summary "Deployment of payment-service v1.2.3" -Source "kubernetes" -Type "deployment_rollout"
Send-Event -Summary "Updated configmap payment-config" -Source "kubernetes" -Type "configmap_update"
Send-Event -Summary "Merged PR #452: Update order processing logic" -Source "git" -Type "pr_merge"
Send-Event -Summary "CI Build #892 succeeded" -Source "ci-cd" -Type "build_success"

Write-Host "Done seeding."
