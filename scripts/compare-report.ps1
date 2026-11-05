<#
.SYNOPSIS
  Compares /reports/daily in sequential and concurrent mode by calling each
  several times and summarising the "took_ms" values the server reports.

.EXAMPLE
  .\scripts\compare-report.ps1
  .\scripts\compare-report.ps1 -Date 2026-11-02 -Runs 50
  .\scripts\compare-report.ps1 -BaseUrl http://localhost:8081
#>
param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$Date = "2026-11-03",
    [int]$Runs = 20
)

$ErrorActionPreference = "Stop"

function Measure-Mode([string]$Mode) {
    $url = "$BaseUrl/reports/daily?date=$Date&mode=$Mode"

    # Warm-up call, not counted: the first request may open new DB connections.
    Invoke-RestMethod $url | Out-Null

    $times = for ($i = 0; $i -lt $Runs; $i++) {
        (Invoke-RestMethod $url).took_ms
    }

    $sorted = $times | Sort-Object
    [pscustomobject]@{
        Mode      = $Mode
        Runs      = $Runs
        MinMs     = [math]::Round(($sorted | Select-Object -First 1), 3)
        MedianMs  = [math]::Round($sorted[[int][math]::Floor($Runs / 2)], 3)
        AverageMs = [math]::Round(($times | Measure-Object -Average).Average, 3)
        MaxMs     = [math]::Round(($sorted | Select-Object -Last 1), 3)
    }
}

Write-Host "Comparing /reports/daily?date=$Date ($Runs runs per mode, after 1 warm-up)..."

# Alternate the modes twice, so a slow moment on the PC doesn't unfairly hit just one.
$results = @(
    Measure-Mode "sequential"
    Measure-Mode "concurrent"
    Measure-Mode "sequential"
    Measure-Mode "concurrent"
)
$results | Format-Table -AutoSize

$seq = ($results | Where-Object Mode -eq "sequential" | Measure-Object MedianMs -Average).Average
$con = ($results | Where-Object Mode -eq "concurrent" | Measure-Object MedianMs -Average).Average
if ($con -gt 0) {
    Write-Host ("Median: sequential {0:N3} ms, concurrent {1:N3} ms -> concurrent is {2:N2}x the speed of sequential" -f $seq, $con, ($seq / $con))
}
