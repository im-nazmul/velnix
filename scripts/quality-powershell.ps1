param(
    [ValidateSet('Lint', 'Format', 'Check')][string]$Mode = 'Lint',
    [Parameter(Mandatory = $true)][string]$AnalyzerPath
)
$ErrorActionPreference = 'Stop'
Import-Module $AnalyzerPath -Force
$projectRoot = Split-Path -Parent $PSScriptRoot
$settingsPath = Join-Path $projectRoot 'tools/PSScriptAnalyzerSettings.psd1'
$scriptFiles = @(
    Get-ChildItem (Join-Path $projectRoot 'scripts') -Filter '*.ps1'
    Get-ChildItem (Join-Path $projectRoot 'tools') -Filter '*.psd1'
)
$failed = $false
foreach ($scriptFile in $scriptFiles) {
    if ($Mode -eq 'Lint') {
        $findings = @(Invoke-ScriptAnalyzer -Path $scriptFile.FullName -Settings $settingsPath)
        if ($findings.Count -gt 0) {
            $findings | Format-Table ScriptName, Line, RuleName, Message -AutoSize | Out-String | Write-Output
            $failed = $true
        }
    }
    else {
        $scriptText = [System.IO.File]::ReadAllText($scriptFile.FullName)
        $formatted = (Invoke-Formatter -ScriptDefinition $scriptText -Settings $settingsPath).Replace("`r`n", "`n").TrimEnd() + "`n"
        if ($Mode -eq 'Format') {
            [System.IO.File]::WriteAllText($scriptFile.FullName, $formatted, [System.Text.UTF8Encoding]::new($false))
        }
        elseif ($formatted -cne $scriptText.Replace("`r`n", "`n")) {
            Write-Output "Needs PowerShell formatting: $($scriptFile.Name)"
            $failed = $true
        }
    }
}
if ($failed) { exit 1 }
