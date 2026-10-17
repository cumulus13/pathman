<#
 End-to-end test of pathman against the REAL Windows registry (CI runner).
 Only touches throw-away user variables named PM_CI_* and a temp backup folder.
#>
param([Parameter(Mandatory)][string]$Exe)

$ErrorActionPreference = 'Stop'
$exe = (Resolve-Path $Exe).Path
$base = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { $env:TEMP }
$tmp = Join-Path $base ("pm-e2e-" + [guid]::NewGuid().ToString('N').Substring(0, 8))
$bk = Join-Path $tmp 'backups'
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$envKey = 'HKCU:\Environment'
$script:failed = 0

# Both helpers take ONE array argument, e.g.  Pm 'edit','PATH','a','b'
# so PowerShell never tries to bind '-e', '-s' or '--dry-run' as its own parameters.
function Pm {
    # runs pathman with its own backup dir; returns exit code + merged output
    param([string[]]$PmArgs)
    $ErrorActionPreference = 'Continue'   # stderr output must not abort the script
    $out = & $exe --no-color --backup-dir $bk @PmArgs 2>&1 | Out-String
    [pscustomobject]@{ Code = $LASTEXITCODE; Out = $out }
}
function PmRaw {
    # same, but WITHOUT forcing --backup-dir (for config tests)
    param([string[]]$PmArgs)
    $ErrorActionPreference = 'Continue'
    $out = & $exe --no-color @PmArgs 2>&1 | Out-String
    [pscustomobject]@{ Code = $LASTEXITCODE; Out = $out }
}
function Raw($name)  { (Get-Item $envKey).GetValue($name, $null, 'DoNotExpandEnvironmentNames') }
function Kind($name) { (Get-Item $envKey).GetValueKind($name).ToString() }
function Count-Backups { @(Get-ChildItem $bk -Filter '*.json' -ErrorAction SilentlyContinue).Count }

function Check($name, [bool]$ok, $detail = '') {
    if ($ok) { Write-Host "  PASS  $name" }
    else {
        Write-Host "  FAIL  $name" -ForegroundColor Red
        if ($detail) { Write-Host "        $detail" -ForegroundColor Red }
        $script:failed++
    }
}

try {
    Write-Host "== config is never read from the current directory"
    $cwd = Join-Path $tmp 'cwd'
    New-Item -ItemType Directory -Force -Path $cwd | Out-Null
    Set-Content -Path (Join-Path $cwd 'pathman.json') -Value '{"backup":{"enabled":false}}'
    Set-Content -Path (Join-Path $cwd 'config.json')  -Value '{"backup":{"enabled":false}}'
    Push-Location $cwd
    $r = PmRaw 'config','show'
    Pop-Location
    Check 'cwd pathman.json ignored' ($r.Code -eq 0 -and $r.Out -match 'Backup\s*:\s*enabled') $r.Out

    $off = Join-Path $tmp 'off.json'
    Set-Content -Path $off -Value '{"backup":{"enabled":false}}'
    $r = PmRaw '--config',$off,'config','show'
    Check '--config file is honoured' ($r.Out -match 'DISABLED') $r.Out

    $bad = Join-Path $tmp 'bad.json'
    Set-Content -Path $bad -Value '{"backup":{"backup_dir":"x"}}'
    $r = PmRaw '--config',$bad,'set','PM_CI_NEVER','1'
    Check 'broken config refuses writes' ($r.Code -ne 0 -and $null -eq (Raw 'PM_CI_NEVER')) $r.Out

    Write-Host "== set / automatic backup"
    $r = Pm 'set','PM_CI_LIST','C:\a;C:\b;C:\c'
    Check 'set works' ($r.Code -eq 0 -and (Raw 'PM_CI_LIST') -eq 'C:\a;C:\b;C:\c') $r.Out
    Check 'a backup file was written' ((Count-Backups) -ge 1)

    Write-Host "== dry-run changes nothing and writes no backup"
    $n = Count-Backups
    $r = Pm '--dry-run','edit','PM_CI_LIST','C:\b','C:\bb'
    Check 'dry-run: exit 0, value unchanged, no new backup' ($r.Code -eq 0 -and (Raw 'PM_CI_LIST') -eq 'C:\a;C:\b;C:\c' -and (Count-Backups) -eq $n) $r.Out

    Write-Host "== edit"
    $r = Pm 'edit','PM_CI_LIST','C:\b','C:\bb'
    Check 'edit replaces one entry in place' ($r.Code -eq 0 -and (Raw 'PM_CI_LIST') -eq 'C:\a;C:\bb;C:\c') $r.Out

    $null = Pm 'set','PM_CI_LUA','c:\SDK\lua;c:\SDK\luarocks;c:\SDK\lua\systree\bin'
    $r = Pm 'edit','PM_CI_LUA','c:\sdk\lua','c:\SDK\lua\55'
    Check 'plain text is whole-entry (luarocks untouched)' ($r.Code -eq 0 -and (Raw 'PM_CI_LUA') -eq 'c:\SDK\lua\55;c:\SDK\luarocks;c:\SDK\lua\systree\bin') (Raw 'PM_CI_LUA')

    $before = Raw 'PM_CI_LUA'
    $r = Pm 'edit','PM_CI_LUA','c:\SDK\lua*','X'
    Check 'multi-entry change needs --confirm' ($r.Code -ne 0 -and (Raw 'PM_CI_LUA') -eq $before) $r.Out

    $r = Pm 'find','C:\bb','-e','PM_CI_LIST'
    Check 'find inside one variable' ($r.Code -eq 0 -and $r.Out -match 'C:\\bb') $r.Out

    Write-Host "== registry value type is preserved (REG_EXPAND_SZ)"
    reg add 'HKCU\Environment' /v PM_CI_EXP /t REG_EXPAND_SZ /d '%SystemRoot%\pm;C:\old' /f | Out-Null
    $r = Pm 'edit','PM_CI_EXP','C:\old','C:\new'
    Check 'edit keeps REG_EXPAND_SZ and %SystemRoot%' ($r.Code -eq 0 -and (Kind 'PM_CI_EXP') -eq 'ExpandString' -and (Raw 'PM_CI_EXP') -eq '%SystemRoot%\pm;C:\new') ("{0} / {1}" -f (Kind 'PM_CI_EXP'), (Raw 'PM_CI_EXP'))

    Write-Host "== restore"
    $r = Pm 'backup','restore','latest','--var','PM_CI_EXP'
    Check 'restore without --confirm only lists' ($r.Code -ne 0 -and (Raw 'PM_CI_EXP') -eq '%SystemRoot%\pm;C:\new') $r.Out
    $r = Pm 'backup','restore','latest','--var','PM_CI_EXP','--confirm'
    Check 'restore --confirm brings old value back, type intact' ($r.Code -eq 0 -and (Kind 'PM_CI_EXP') -eq 'ExpandString' -and (Raw 'PM_CI_EXP') -eq '%SystemRoot%\pm;C:\old') ("{0} / {1}`n{2}" -f (Kind 'PM_CI_EXP'), (Raw 'PM_CI_EXP'), $r.Out)

    $null = Pm 'set','PM_CI_R','C:\one'
    $null = Pm 'set','PM_CI_R','C:\two'
    $r = Pm 'backup','restore','latest','--var','PM_CI_R','--confirm'
    Check 'restore undoes the last set' ($r.Code -eq 0 -and (Raw 'PM_CI_R') -eq 'C:\one') $r.Out

    Write-Host "== backup failure must block the change"
    $notDir = Join-Path $tmp 'iam-a-file'
    Set-Content -Path $notDir -Value 'x'
    $keep = Raw 'PM_CI_LIST'
    $ErrorActionPreference = 'Continue'
    $out = & $exe --no-color --backup-dir $notDir set PM_CI_LIST 'C:\zzz' 2>&1 | Out-String
    $code = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
    Check 'unwritable backup dir: command fails, value unchanged' ($code -ne 0 -and (Raw 'PM_CI_LIST') -eq $keep) $out

    Write-Host "== --no-backup and delete"
    $n = Count-Backups
    $r = Pm '--no-backup','set','PM_CI_NB','1'
    Check '--no-backup writes no backup' ($r.Code -eq 0 -and (Count-Backups) -eq $n -and (Raw 'PM_CI_NB') -eq '1') $r.Out
    $r = Pm 'delete','PM_CI_NB'
    Check 'delete removes the variable' ($r.Code -eq 0 -and $null -eq (Raw 'PM_CI_NB')) $r.Out
    Check 'delete took a backup first' ((Count-Backups) -gt $n)

    Write-Host "== backup list / dir"
    $r = Pm 'backup','list'
    Check 'backup list shows snapshots' ($r.Code -eq 0 -and $r.Out -match '\.json') $r.Out
    $r = Pm 'backup','dir'
    Check 'backup dir prints the --backup-dir' ($r.Code -eq 0 -and $r.Out.Trim() -eq $bk) $r.Out
}
finally {
    foreach ($v in 'PM_CI_LIST', 'PM_CI_LUA', 'PM_CI_EXP', 'PM_CI_R', 'PM_CI_NB', 'PM_CI_NEVER') {
        reg delete 'HKCU\Environment' /v $v /f 2>$null | Out-Null
    }
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    # `reg delete` of a variable that was never created exits 1; the CI shell would
    # report that stale code as the script result.
    $global:LASTEXITCODE = 0
}

if ($script:failed -gt 0) {
    Write-Host "`n$($script:failed) e2e check(s) FAILED" -ForegroundColor Red
    exit 1
}
Write-Host "`nAll e2e checks passed"
exit 0
