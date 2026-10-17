<#
 End-to-end test of the NSIS installers on a real Windows runner.
 Installs silently, checks the system PATH (long real-world PATH, value type,
 other entries untouched), upgrades in place, uninstalls, and checks that
 dangerous install folders are refused.
#>
param([Parameter(Mandatory)][string]$SetupDir)

$ErrorActionPreference = 'Stop'
$sysKey = 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment'
$script:failed = 0

function RawPath  { (Get-Item $sysKey).GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames) }
function PathKind { (Get-Item $sysKey).GetValueKind('Path').ToString() }
function Entries($p) { @($p -split ';' | Where-Object { $_ -ne '' }) }
function Same-List($a, $b) { ((@($a) -join '|') -ieq (@($b) -join '|')) }

function Check($name, [bool]$ok, $detail = '') {
    if ($ok) { Write-Host "  PASS  $name" }
    else {
        Write-Host "  FAIL  $name" -ForegroundColor Red
        if ($detail) { Write-Host "        $detail" -ForegroundColor Red }
        $script:failed++
    }
}

function Run-Setup($file, [string[]]$arguments) {
    $p = Start-Process -FilePath $file -ArgumentList $arguments -Wait -PassThru
    return $p.ExitCode
}

function Test-Installer($setup, $dir, $uninstKey, $label) {
    Write-Host "`n== $label  ($([IO.Path]::GetFileName($setup)))"
    $pre = RawPath
    $preKind = PathKind
    $preEntries = Entries $pre
    Write-Host "   PATH before: $($pre.Length) chars, $($preEntries.Count) entries, $preKind"

    # ── install ──
    $rc = Run-Setup $setup @('/S')
    Check 'install exits 0' ($rc -eq 0) "exit code $rc"
    Check 'pathman.exe installed' (Test-Path "$dir\pathman.exe")
    Check 'README, LICENSE, uninstall.exe installed' ((Test-Path "$dir\README.md") -and (Test-Path "$dir\LICENSE") -and (Test-Path "$dir\uninstall.exe"))

    $post = RawPath
    $postEntries = Entries $post
    $ours = @($postEntries | Where-Object { $_.TrimEnd('\') -ieq $dir })
    $others = @($postEntries | Where-Object { $_.TrimEnd('\') -ine $dir })
    Check 'install folder is on the system PATH exactly once' ($ours.Count -eq 1) "count = $($ours.Count)"
    Check 'every other PATH entry is unchanged and in the same order' (Same-List $others $preEntries) "pre=$($preEntries.Count) now=$($others.Count)"
    Check 'PATH registry type is unchanged' ((PathKind) -eq $preKind) "was $preKind now $(PathKind)"
    Check 'long PATH was not truncated' ($post.Length -ge $pre.Length) "$($pre.Length) -> $($post.Length)"
    Check 'Add/Remove Programs entry exists' (Test-Path $uninstKey)
    Check 'automatic system backup was written' (@(Get-ChildItem "$env:LOCALAPPDATA\pathman\backups" -Filter '*_system.json' -ErrorAction SilentlyContinue).Count -ge 1)

    & "$dir\pathman.exe" --version | Out-Null
    Check 'installed pathman.exe runs' ($LASTEXITCODE -eq 0)

    # ── upgrade in place ──
    $rc = Run-Setup $setup @('/S')
    Check 'second install (upgrade) exits 0' ($rc -eq 0) "exit code $rc"
    Check 'PATH still contains the folder exactly once' (@(Entries (RawPath) | Where-Object { $_.TrimEnd('\') -ieq $dir }).Count -eq 1)

    # ── uninstall ──
    # NSIS uninstallers re-launch a copy of themselves from %TEMP% and return at
    # once, so poll for the result instead of trusting -Wait.
    Start-Process -FilePath "$dir\uninstall.exe" -ArgumentList '/S' -Wait | Out-Null
    $deadline = (Get-Date).AddSeconds(90)
    while ((Get-Date) -lt $deadline -and ((Test-Path "$dir\pathman.exe") -or (Test-Path $uninstKey))) { Start-Sleep -Seconds 1 }

    Check 'uninstall removed the program files' (-not (Test-Path "$dir\pathman.exe"))
    Check 'uninstall removed the Add/Remove Programs entry' (-not (Test-Path $uninstKey))
    Check 'PATH entries are identical to before the install' (Same-List (Entries (RawPath)) $preEntries) "now=$((Entries (RawPath)).Count) expected=$($preEntries.Count)"
    Check 'PATH registry type is still unchanged' ((PathKind) -eq $preKind)
    Check 'backups were kept' (@(Get-ChildItem "$env:LOCALAPPDATA\pathman\backups" -Filter '*.json' -ErrorAction SilentlyContinue).Count -ge 1)
    if (Test-Path $dir) { Remove-Item -Recurse -Force $dir -ErrorAction SilentlyContinue }
}

try {
    $amd64 = Get-ChildItem $SetupDir -Filter '*_windows_amd64_setup.exe' -Recurse | Select-Object -First 1
    $x86   = Get-ChildItem $SetupDir -Filter '*_windows_386_setup.exe'   -Recurse | Select-Object -First 1
    if (-not $amd64 -or -not $x86) { throw "installers not found under $SetupDir" }

    Test-Installer $amd64.FullName 'C:\Program Files\PathMan' `
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PathMan' 'amd64'
    Test-Installer $x86.FullName 'C:\Program Files (x86)\PathMan' `
        'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\PathMan' '386'

    Write-Host "`n== custom folder and refused folders"
    $before = Entries (RawPath)
    $custom = 'C:\PathManCI'
    $rc = Run-Setup $amd64.FullName @('/S', "/D=$custom")
    Check 'custom folder via /D= installs' ($rc -eq 0 -and (Test-Path "$custom\pathman.exe")) "exit code $rc"
    Check 'custom folder is on PATH' (@(Entries (RawPath) | Where-Object { $_.TrimEnd('\') -ieq $custom }).Count -eq 1)
    Start-Process -FilePath "$custom\uninstall.exe" -ArgumentList '/S' -Wait | Out-Null
    $deadline = (Get-Date).AddSeconds(90)
    while ((Get-Date) -lt $deadline -and (Test-Path "$custom\pathman.exe")) { Start-Sleep -Seconds 1 }
    Start-Sleep -Seconds 3
    Check 'custom install removed and PATH restored' ((-not (Test-Path "$custom\pathman.exe")) -and (Same-List (Entries (RawPath)) $before))
    if (Test-Path $custom) { Remove-Item -Recurse -Force $custom -ErrorAction SilentlyContinue }

    foreach ($bad in 'C:\Bad;Dir', 'C:\Bad[x]', 'C:\Bad]x') {
        $rc = Run-Setup $amd64.FullName @('/S', "/D=$bad")
        Check "folder '$bad' is refused (exit 2), nothing changed" ($rc -eq 2 -and (Same-List (Entries (RawPath)) $before) -and -not (Test-Path 'C:\Bad*')) "exit code $rc"
    }
}
finally {
    foreach ($d in 'C:\Program Files\PathMan', 'C:\Program Files (x86)\PathMan', 'C:\PathManCI') {
        if (Test-Path "$d\uninstall.exe") {
            Start-Process -FilePath "$d\uninstall.exe" -ArgumentList '/S' -Wait | Out-Null
        }
    }
}

if ($script:failed -gt 0) {
    Write-Host "`n$($script:failed) installer check(s) FAILED" -ForegroundColor Red
    exit 1
}
Write-Host "`nAll installer checks passed"
