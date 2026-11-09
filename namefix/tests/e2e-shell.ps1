#Requires -Version 7.0
<#
.SYNOPSIS
  탐색기 오른쪽 클릭 메뉴 연동 검증입니다(install.bat 실행 후 사용).

.DESCRIPTION
  셸 COM 개체(Shell.Application)로 등록된 메뉴 동사를 실제로 호출합니다.
  탐색기에서 항목을 선택하고 메뉴를 누를 때와 같은 경로(IContextMenu)를 거칩니다.
    1) 파일 20개 다중 선택 → "파일명 정리" (namefix)
       기본 다중 선택 모델에서는 15개를 넘으면 메뉴가 숨겨지므로 20개로 확인합니다.
    2) 폴더 선택 → "폴더 이름 정리" (namefix)
    3) 폴더 선택 → "폴더와 하위 항목 이름 모두 정리" (namefix.recursive)
    4) 폴더 빈 공간 → "이 폴더의 파일명 정리": 등록된 명령줄을 그대로 실행

  레지스트리는 읽기만 합니다. 임시 폴더만 변경합니다.

.PARAMETER Keep
  테스트가 끝난 뒤 임시 폴더를 지우지 않습니다.
#>
param([switch]$Keep)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ([Threading.Thread]::CurrentThread.GetApartmentState() -ne 'STA') {
    throw '셸 COM 호출에는 STA 스레드가 필요합니다. pwsh -STA로 실행하십시오.'
}

$script:passed = 0
$script:failed = [System.Collections.Generic.List[string]]::new()
function Test-Case([string]$Name, [bool]$Condition, [string]$Detail = '') {
    if ($Condition) { $script:passed++; Write-Host "PASS  $Name" }
    else {
        $script:failed.Add($Name)
        Write-Host "FAIL  $Name" -ForegroundColor Red
        if ($Detail) { Write-Host "      $Detail" -ForegroundColor DarkYellow }
    }
}
function ConvertTo-Nfd([string]$Text) { $Text.Normalize([Text.NormalizationForm]::FormD) }
function Get-Names([string]$Dir) { @([IO.Directory]::GetFileSystemEntries($Dir) | ForEach-Object { [IO.Path]::GetFileName($_) }) }
function Test-HasName([string]$Dir, [string]$Name) {
    if (-not [IO.Directory]::Exists($Dir)) { return $false }
    foreach ($n in (Get-Names $Dir)) { if ([string]::Equals($n, $Name, [StringComparison]::Ordinal)) { return $true } }
    $false
}
# 조건이 참이 될 때까지 최대 TimeoutSec초 기다립니다. 메뉴 동사는 비동기로 프로세스를 실행합니다.
function Wait-Until([scriptblock]$Condition, [int]$TimeoutSec = 30) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSec)
    while ([DateTime]::UtcNow -lt $deadline) {
        if (& $Condition) { return $true }
        Start-Sleep -Milliseconds 200
    }
    [bool](& $Condition)
}
# HKCU\Software\Classes 아래 메뉴 명령줄을 읽습니다. 키 이름의 *는 와일드카드가 아니므로 .NET API로 읽습니다.
function Get-VerbCommand([string]$KeyPath) {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Software\Classes\$KeyPath\command")
    if ($null -eq $key) { return $null }
    try { $key.GetValue('') } finally { $key.Dispose() }
}

# ---------------------------------------------------------------------------
# 0. 등록 상태
# ---------------------------------------------------------------------------
$verbs = [ordered]@{
    'file'       = '*\shell\namefix'
    'folder'     = 'Directory\shell\namefix'
    'recursive'  = 'Directory\shell\namefix.recursive'
    'background' = 'Directory\Background\shell\namefix'
}
$commands = @{}
foreach ($k in $verbs.Keys) {
    $commands[$k] = Get-VerbCommand $verbs[$k]
    $exe = if ($commands[$k] -match '^"([^"]+)"') { $Matches[1] } else { $null }
    Test-Case "등록: $($verbs[$k]) 명령이 있고 실행 파일이 존재한다" ($null -ne $exe -and (Test-Path -LiteralPath $exe)) "$($commands[$k])"
}
if ($script:failed.Count -gt 0) {
    Write-Host '메뉴가 등록되어 있지 않습니다. install.bat을 먼저 실행하십시오.'
    exit 1
}

$work = Join-Path ([IO.Path]::GetTempPath()) ('namefix-e2e-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
[void][IO.Directory]::CreateDirectory($work)
$logPath = Join-Path $work 'namefix.log'
# 셸이 실행하는 프로세스는 이 프로세스의 환경 변수를 물려받습니다.
$env:NAMEFIX_LOG = $logPath
$env:NAMEFIX_NO_DIALOG = '1'
$shell = New-Object -ComObject Shell.Application
Write-Host "작업 폴더: $work"

# ---------------------------------------------------------------------------
# 1. 파일 20개 다중 선택
# ---------------------------------------------------------------------------
$d = Join-Path $work 'multi'
[void][IO.Directory]::CreateDirectory($d)
$names = 1..20 | ForEach-Object { '다중 선택 {0:D2}.txt' -f $_ }
foreach ($n in $names) { [IO.File]::WriteAllText((Join-Path $d (ConvertTo-Nfd $n)), $n) }
$items = $shell.Namespace($d).Items()
$items.Filter(0x40 -bor 0x80, '*')   # SHCONTF_NONFOLDERS | SHCONTF_INCLUDEHIDDEN
Test-Case '파일 20개를 한 번에 선택했다' ($items.Count -eq 20) "count=$($items.Count)"
$items.InvokeVerbEx('namefix')
$done = Wait-Until { foreach ($n in $names) { if (-not (Test-HasName $d $n)) { return $false } }; $true }
$remaining = @(Get-Names $d | Where-Object { -not $_.IsNormalized([Text.NormalizationForm]::FormC) }).Count
Test-Case '다중 선택 메뉴로 20개 모두 이름을 정리한다' $done "NFD로 남은 항목 $remaining 개"
Start-Sleep -Milliseconds 800
$summaries = @(if (Test-Path -LiteralPath $logPath) { Get-Content -LiteralPath $logPath -Encoding utf8 | Where-Object { $_ -match "`tSUMMARY`t" } })
Write-Host ("      실행 요약 {0}건: {1}" -f $summaries.Count, (($summaries | ForEach-Object { ($_ -split "`t")[2..4] -join ' ' }) -join ' | '))
Test-Case '다중 선택 요청을 한 번에 모아 처리한다(결과 1회)' ($summaries.Count -eq 1) ($summaries -join ' | ')

# ---------------------------------------------------------------------------
# 2. 폴더 선택 → 폴더 이름만 정리
# ---------------------------------------------------------------------------
$d = Join-Path $work 'folder'
[void][IO.Directory]::CreateDirectory($d)
$folderNfd = ConvertTo-Nfd '폴더 선택'
$innerNfd = ConvertTo-Nfd '안쪽.txt'
[void][IO.Directory]::CreateDirectory((Join-Path $d $folderNfd))
[IO.File]::WriteAllText((Join-Path (Join-Path $d $folderNfd) $innerNfd), 'x')
$item = $shell.Namespace($d).ParseName($folderNfd)
$item.InvokeVerb('namefix')
$done = Wait-Until { Test-HasName $d '폴더 선택' }
Test-Case '폴더 메뉴로 폴더 이름을 정리하고 안의 항목은 그대로 둔다' ($done -and (Test-HasName (Join-Path $d '폴더 선택') $innerNfd)) ((Get-Names $d) -join ', ')

# ---------------------------------------------------------------------------
# 3. 폴더 선택 → 폴더와 하위 항목 모두 정리
# ---------------------------------------------------------------------------
$d = Join-Path $work 'recursive'
[void][IO.Directory]::CreateDirectory($d)
$top = Join-Path $d (ConvertTo-Nfd '전체 정리')
$sub = Join-Path $top (ConvertTo-Nfd '하위')
[void][IO.Directory]::CreateDirectory($sub)
[IO.File]::WriteAllText((Join-Path $sub (ConvertTo-Nfd '깊은 파일.txt')), 'x')
$item = $shell.Namespace($d).ParseName((ConvertTo-Nfd '전체 정리'))
$item.InvokeVerb('namefix.recursive')
$done = Wait-Until { Test-HasName (Join-Path $d '전체 정리\하위') '깊은 파일.txt' }
Test-Case '하위 포함 메뉴로 폴더와 모든 하위 항목 이름을 정리한다' ($done -and (Test-HasName $d '전체 정리')) ((Get-Names $d) -join ', ')

# ---------------------------------------------------------------------------
# 4. 폴더 빈 공간 메뉴: 등록된 명령줄의 %V를 폴더 경로로 바꿔 실행
# ---------------------------------------------------------------------------
$d = Join-Path $work 'background'
[void][IO.Directory]::CreateDirectory($d)
$names = '빈 공간 1.txt', '빈 공간 2.txt'
foreach ($n in $names) { [IO.File]::WriteAllText((Join-Path $d (ConvertTo-Nfd $n)), 'x') }
if ($commands['background'] -match '^"([^"]+)"\s+(.*)$') {
    $proc = Start-Process -FilePath $Matches[1] -ArgumentList ($Matches[2].Replace('%V', $d)) -Wait -PassThru
    $ok = $true
    foreach ($n in $names) { if (-not (Test-HasName $d $n)) { $ok = $false } }
    Test-Case '빈 공간 메뉴 명령으로 현재 폴더 안 항목을 정리한다' ($proc.ExitCode -eq 0 -and $ok) ((Get-Names $d) -join ', ')
}
else {
    Test-Case '빈 공간 메뉴 명령 형식을 해석한다' $false $commands['background']
}

# ---------------------------------------------------------------------------
# 결과
# ---------------------------------------------------------------------------
Write-Host ''
Write-Host ("결과: 통과 {0}건, 실패 {1}건" -f $script:passed, $script:failed.Count)
Remove-Item Env:\NAMEFIX_LOG, Env:\NAMEFIX_NO_DIALOG -ErrorAction SilentlyContinue
if ($script:failed.Count -eq 0 -and -not $Keep) { Remove-Item -LiteralPath $work -Recurse -Force }
else { Write-Host "테스트 폴더를 남겼습니다: $work" }
exit ([int]($script:failed.Count -gt 0))
