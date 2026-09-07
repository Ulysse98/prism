$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Assert-NativeSuccess {
    param(
        [string]$Step
    )

    if ($LASTEXITCODE -ne 0) {
        throw "$Step failed with exit code $LASTEXITCODE"
    }
}

function Get-PrismTip {
    param(
        [string]$Name,
        [string]$Container,
        [int]$Port
    )

    $json = docker exec $Container `
        cat "/app/data/node-$Port/chain.json"

    Assert-NativeSuccess "Reading $Name chain"

    $state = $json | ConvertFrom-Json

    if ($null -eq $state.blockchain) {
        throw "$Name blockchain is missing"
    }

    if ($state.blockchain.blocks.Count -eq 0) {
        throw "$Name has no blocks"
    }

    $last = $state.blockchain.blocks[-1]

    return [PSCustomObject]@{
        Name   = $Name
        Height = [uint64]$last.height
        Hash   = [string]$last.hash
    }
}

function Assert-Converged {
    param(
        [array]$Tips,
        [string]$Stage
    )

    $heights = @(
        $Tips |
            Select-Object -ExpandProperty Height -Unique
    )

    $hashes = @(
        $Tips |
            Select-Object -ExpandProperty Hash -Unique
    )

    if ($heights.Count -ne 1) {
        throw "${Stage}: nodes have different heights"
    }

    if ($hashes.Count -ne 1) {
        throw "${Stage}: nodes have different hashes"
    }

    if ([string]::IsNullOrWhiteSpace($hashes[0])) {
        throw "${Stage}: empty block hash"
    }
}

$nodes = @(
    @{
        Name      = "node-1"
        Container = "prism-node-1"
        Port      = 7001
    },
    @{
        Name      = "node-2"
        Container = "prism-node-2"
        Port      = 7002
    },
    @{
        Name      = "node-3"
        Container = "prism-node-3"
        Port      = 7003
    }
)

Write-Host ""
Write-Host "=== PRISM v0.19 NETWORKING INTEGRATION TEST ==="
Write-Host ""

Write-Host "Rebuilding devnet..."
docker compose down
Assert-NativeSuccess "docker compose down"

docker compose build
Assert-NativeSuccess "docker compose build"

docker compose up -d
Assert-NativeSuccess "docker compose up"

Write-Host ""
Write-Host "Waiting for P2P mesh..."
Start-Sleep -Seconds 15

Write-Host ""
Write-Host "=== INITIAL TIPS ==="

$before = @()

foreach ($node in $nodes) {
    $tip = Get-PrismTip `
        -Name $node.Name `
        -Container $node.Container `
        -Port $node.Port

    $before += $tip

    Write-Host (
        "{0}: height={1} hash={2}" -f
        $tip.Name,
        $tip.Height,
        $tip.Hash
    )
}

Assert-Converged `
    -Tips $before `
    -Stage "Initial state"

$initialHeight = $before[0].Height
$expectedHeight = $initialHeight + 1

Write-Host ""
Write-Host "Initial network converged at height $initialHeight."

Write-Host ""
Write-Host "=== PRODUCING BLOCK ==="

$produceOutput = @(
    docker exec prism-node-1 prism node-produce `
        --port 7001 `
        --peer prism-node-2:7002 2>&1
)

$produceOutput | Out-Host

Assert-NativeSuccess "node-produce"

$accepted = $produceOutput |
    Select-String -SimpleMatch "Block submission: ACCEPTED"

if ($null -eq $accepted) {
    throw "Produced block was not accepted by the P2P peer"
}

Write-Host ""
Write-Host "Waiting for block gossip..."
Start-Sleep -Seconds 5

Write-Host ""
Write-Host "=== FINAL TIPS ==="

$after = @()

foreach ($node in $nodes) {
    $tip = Get-PrismTip `
        -Name $node.Name `
        -Container $node.Container `
        -Port $node.Port

    $after += $tip

    Write-Host (
        "{0}: height={1} hash={2}" -f
        $tip.Name,
        $tip.Height,
        $tip.Hash
    )
}

Assert-Converged `
    -Tips $after `
    -Stage "Final state"

$finalHeight = $after[0].Height
$finalHash = $after[0].Hash

if ($finalHeight -ne $expectedHeight) {
    throw (
        "Expected height {0}, got {1}" -f
        $expectedHeight,
        $finalHeight
    )
}

Write-Host ""
Write-Host "=== CHECKING P2P ERRORS ==="

$networkErrors = @()

foreach ($node in $nodes) {
    $logs = docker logs $node.Container 2>&1

    Assert-NativeSuccess "Reading $($node.Name) logs"

    $errors = @(
        $logs |
            Select-String `
                "Block rejected|Block broadcast failed"
    )

    foreach ($errorLine in $errors) {
        $networkErrors += (
            "{0}: {1}" -f
            $node.Name,
            $errorLine.Line
        )
    }
}

if ($networkErrors.Count -gt 0) {
    Write-Host ""
    Write-Host "Networking errors detected:"

    $networkErrors | ForEach-Object {
        Write-Host $_
    }

    throw "Block propagation produced network errors"
}

Write-Host "No block propagation errors detected."

Write-Host ""
Write-Host "============================================"
Write-Host " PRISM v0.19 NETWORKING TEST: PASS"
Write-Host "============================================"
Write-Host "Height: $finalHeight"
Write-Host "Hash:   $finalHash"
Write-Host "Nodes:  3/3 converged"
Write-Host ""

