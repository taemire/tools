#Requires -Version 7.0
<#
.SYNOPSIS
  namefix 회귀 테스트입니다.

.DESCRIPTION
  임시 폴더에 자소 분리(NFD)·URL 인코딩 이름을 가진 파일과 폴더를 만들고
  bin\namefix.exe(콘솔용)와 bin\namefixw.exe(탐색기용)를 실행한 뒤,
  실제 파일 시스템에 남은 이름으로 결과를 판정합니다.

  주의: PowerShell의 -eq, -contains는 문화권 비교를 사용하므로 NFD와 NFC를
  같은 문자열로 판정할 수 있습니다. 이름 비교는 반드시 서수(Ordinal) 비교로 수행합니다.

  레지스트리(탐색기 메뉴)는 건드리지 않습니다. 메뉴 연동 검증은 tests\e2e-shell.ps1에서 수행합니다.

.PARAMETER SkipBuild
  build.bat 실행을 건너뛰고 기존 bin\ 실행 파일로 테스트합니다.

.PARAMETER Keep
  테스트가 끝난 뒤 임시 폴더를 지우지 않습니다(실패 분석용).

.EXAMPLE
  pwsh -NoProfile -File tests\run-tests.ps1
#>
param(
    [switch]$SkipBuild,
    [switch]$Keep
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$cli = Join-Path $root 'bin\namefix.exe'
$gui = Join-Path $root 'bin\namefixw.exe'

if (-not $SkipBuild) {
    & (Join-Path $root 'build.bat')
    if ($LASTEXITCODE -ne 0) { throw "build.bat 실패(종료 코드 $LASTEXITCODE)" }
}
foreach ($exe in $cli, $gui) {
    if (-not (Test-Path -LiteralPath $exe)) { throw "실행 파일이 없습니다: $exe" }
}

# 테스트 전용 작업 폴더와 로그 경로입니다. 실제 사용자 로그(%LOCALAPPDATA%\namefix)를 오염시키지 않습니다.
$work = Join-Path ([IO.Path]::GetTempPath()) ('namefix-test-' + [guid]::NewGuid().ToString('N').Substring(0, 8))
[void][IO.Directory]::CreateDirectory($work)
$logPath = Join-Path $work 'namefix.log'
$env:NAMEFIX_LOG = $logPath
# 대화상자 대신 종료 코드와 로그로 판정할 수 있도록 대화상자를 끕니다.
$env:NAMEFIX_NO_DIALOG = '1'

$script:passed = 0
$script:failed = [System.Collections.Generic.List[string]]::new()

# 판정 결과를 집계하고 출력합니다.
function Test-Case([string]$Name, [bool]$Condition, [string]$Detail = '') {
    if ($Condition) {
        $script:passed++
        Write-Host "PASS  $Name"
    }
    else {
        $script:failed.Add($Name)
        Write-Host "FAIL  $Name" -ForegroundColor Red
        if ($Detail) { Write-Host "      $Detail" -ForegroundColor DarkYellow }
    }
}

# 문자열을 자소 분리(NFD) 형태로 바꿉니다. macOS에서 넘어온 파일명과 같은 형태입니다.
function ConvertTo-Nfd([string]$Text) { $Text.Normalize([Text.NormalizationForm]::FormD) }

# 테스트 케이스별 폴더를 만듭니다. 이름에 와일드카드 문자가 있어도 안전하도록 .NET API를 씁니다.
function New-CaseDir([string]$Name) {
    $dir = Join-Path $work $Name
    [void][IO.Directory]::CreateDirectory($dir)
    $dir
}

function New-TestFile([string]$Dir, [string]$Name, [string]$Content = 'namefix') {
    $path = [IO.Path]::Combine($Dir, $Name)
    [IO.File]::WriteAllText($path, $Content)
    $path
}

function New-TestDir([string]$Dir, [string]$Name) {
    $path = [IO.Path]::Combine($Dir, $Name)
    [void][IO.Directory]::CreateDirectory($path)
    $path
}

# 폴더 안 항목 이름 목록(디스크에 저장된 형태 그대로)을 돌려줍니다.
function Get-Names([string]$Dir) {
    # 없는 폴더는 빈 목록입니다(실패 메시지를 만들 때 예외로 테스트가 중단되지 않도록).
    if (-not [IO.Directory]::Exists($Dir)) { return @() }
    @([IO.Directory]::GetFileSystemEntries($Dir) | ForEach-Object { [IO.Path]::GetFileName($_) })
}

# 서수 비교로 이름이 정확히 존재하는지 확인합니다.
function Test-HasName([string]$Dir, [string]$Name) {
    if (-not [IO.Directory]::Exists($Dir)) { return $false }
    foreach ($n in (Get-Names $Dir)) {
        if ([string]::Equals($n, $Name, [StringComparison]::Ordinal)) { return $true }
    }
    $false
}

function Read-TextOrNull([string]$Path) {
    try { [IO.File]::ReadAllText($Path) } catch { $null }
}

# 콘솔용 실행 파일을 호출하고 종료 코드와 출력을 돌려줍니다.
function Invoke-Cli {
    $ErrorActionPreference = 'Continue'
    $output = & $cli @args 2>&1 | Out-String
    [pscustomobject]@{ Code = $LASTEXITCODE; Output = $output }
}

# 로그에서 SUMMARY 줄만 읽습니다.
function Get-SummaryLines {
    if (-not (Test-Path -LiteralPath $logPath)) { return @() }
    @(Get-Content -LiteralPath $logPath -Encoding utf8 | Where-Object { $_ -match "`tSUMMARY`t" })
}

Write-Host "작업 폴더: $work"
Write-Host ''

# ---------------------------------------------------------------------------
# 0. 전제: NFD와 NFC가 서수 비교로 서로 다른 문자열이어야 테스트가 의미가 있습니다.
# ---------------------------------------------------------------------------
Test-Case '전제: NFD와 NFC는 서수 비교로 다르다' (-not [string]::Equals('한글', (ConvertTo-Nfd '한글'), [StringComparison]::Ordinal))

# ---------------------------------------------------------------------------
# 1. 자소 분리(NFD) 파일명
# ---------------------------------------------------------------------------
$d = New-CaseDir 'nfd-file'
$nfc = '웨비나(배포본) 자료.pdf'
$nfd = ConvertTo-Nfd $nfc
$p = New-TestFile $d $nfd 'payload-1'
$r = Invoke-Cli $p
Test-Case 'NFD 파일명을 NFC로 바꾼다' ($r.Code -eq 0 -and (Test-HasName $d $nfc) -and -not (Test-HasName $d $nfd)) $r.Output
Test-Case '이름만 바꾸고 내용은 유지한다' ((Read-TextOrNull (Join-Path $d $nfc)) -eq 'payload-1')

# ---------------------------------------------------------------------------
# 2~4. URL 인코딩 이름
# ---------------------------------------------------------------------------
$d = New-CaseDir 'url-korean'
$enc = [Uri]::EscapeDataString('한글 문서') + '.pdf'
$p = New-TestFile $d $enc
$r = Invoke-Cli $p
Test-Case 'URL 인코딩된 한글 이름을 디코딩한다' ($r.Code -eq 0 -and (Test-HasName $d '한글 문서.pdf') -and @(Get-Names $d).Count -eq 1) $r.Output

$d = New-CaseDir 'url-nfd'
$enc = [Uri]::EscapeDataString((ConvertTo-Nfd '자료')) + '.txt'
$p = New-TestFile $d $enc
$r = Invoke-Cli $p
Test-Case 'NFD 바이트가 URL 인코딩된 이름은 디코딩 후 NFC로 합친다' ($r.Code -eq 0 -and (Test-HasName $d '자료.txt')) $r.Output

$d = New-CaseDir 'url-double'
$enc = [Uri]::EscapeDataString([Uri]::EscapeDataString('보고서')) + '.docx'
$p = New-TestFile $d $enc
$r = Invoke-Cli $p
Test-Case '이중 URL 인코딩을 끝까지 디코딩한다' ($r.Code -eq 0 -and (Test-HasName $d '보고서.docx')) $r.Output

# ---------------------------------------------------------------------------
# 5. 그대로 두어야 하는 이름
# ---------------------------------------------------------------------------
$d = New-CaseDir 'keep'
$keepNames = @(
    '100% 완료.txt',      # %와 공백: 16진수 두 자리가 아님
    '50%OFF.txt',         # %OF: 16진수가 아님
    'a%2Fb%3Fc.txt',      # / ? 는 파일명에 쓸 수 없으므로 인코딩 유지
    '%FF%FE.txt',         # UTF-8로 해석할 수 없는 바이트
    'abc%E3.txt',         # 끊긴 UTF-8 시퀀스
    'C++ guide.txt',      # +는 공백으로 바꾸지 않음
    'name%20',            # 디코딩하면 끝이 공백인 이름(Windows에서 사용 불가)
    'CON%2Etxt',          # 디코딩하면 예약 장치 이름(CON.txt)
    'COM%C2%B9.txt',      # 디코딩하면 예약 장치 이름(COM¹.txt, 위 첨자 숫자)
    'lpt%C2%B3.log',      # 디코딩하면 예약 장치 이름(lpt³.log, 대소문자 무시)
    '정상 이름.txt'       # 이미 정상
)
$paths = foreach ($n in $keepNames) { New-TestFile $d $n }
$r = Invoke-Cli @paths
$allKept = $true
foreach ($n in $keepNames) { if (-not (Test-HasName $d $n)) { $allKept = $false } }
Test-Case '디코딩하면 안 되거나 이미 정상인 이름은 그대로 둔다' ($r.Code -eq 0 -and $allKept -and @(Get-Names $d).Count -eq $keepNames.Count) $r.Output

# ---------------------------------------------------------------------------
# 6. 일부만 디코딩
# ---------------------------------------------------------------------------
$d = New-CaseDir 'partial'
$p1 = New-TestFile $d '%41%2Fx.txt'
$p2 = New-TestFile $d '%ED%95%9C%3A%EA%B8%80.txt'
$r = Invoke-Cli $p1 $p2
Test-Case '파일명에 쓸 수 없는 문자만 인코딩 상태로 남기고 나머지는 디코딩한다' ($r.Code -eq 0 -and (Test-HasName $d 'A%2Fx.txt') -and (Test-HasName $d '한%3A글.txt')) $r.Output

# URL 디코딩 결과를 쓸 수 없어도 NFC 정규화는 적용합니다.
$d = New-CaseDir 'nfc-even-if-url-skipped'
$name = (ConvertTo-Nfd '자료') + '%20'
$p = New-TestFile $d $name
$r = Invoke-Cli $p
Test-Case 'URL 디코딩을 건너뛰어도 NFC 정규화는 적용한다' ($r.Code -eq 0 -and (Test-HasName $d '자료%20')) $r.Output

# COM0·LPT0은 Windows 예약 이름 목록(COM1~9, COM¹~³, LPT1~9, LPT¹~³)에 없으므로 유효한 디코딩 결과입니다.
$d = New-CaseDir 'not-reserved'
$p1 = New-TestFile $d 'COM0%2Elog'
$p2 = New-TestFile $d 'LPT0%2Etxt'
$r = Invoke-Cli $p1 $p2
Test-Case 'COM0·LPT0은 예약 이름이 아니므로 디코딩한다' ($r.Code -eq 0 -and (Test-HasName $d 'COM0.log') -and (Test-HasName $d 'LPT0.txt')) $r.Output

# 전체 변환(디코딩·유효성 검사·NFC)의 멱등성: 첫 변환의 NFC가 결합 문자를 합치면 다음 변환에서 디코딩 가능 여부가 달라지는 이름
$d = New-CaseDir 'normalize-fixpoint'
$tricky = "CON%2E$([char]0x0301)%41.txt"
[void](New-TestFile $d $tricky)
$r1 = Invoke-Cli -c $d
$after1 = @(Get-Names $d)
$r2 = Invoke-Cli -c $d
$after2 = @(Get-Names $d)
Test-Case '전체 변환을 고정점까지 적용하여 다시 실행해도 이름이 더 바뀌지 않는다' ($r1.Code -eq 0 -and $r2.Code -eq 0 -and $r2.Output -match '변경 0' -and [string]::Equals(($after1 -join '|'), ($after2 -join '|'), [StringComparison]::Ordinal)) ("1차=" + ($after1 -join ',') + " 2차=" + ($after2 -join ',') + " | " + $r2.Output)

# ---------------------------------------------------------------------------
# 7. 충돌
# ---------------------------------------------------------------------------
$d = New-CaseDir 'conflict'
$nfc = '충돌 문서.txt'
$nfd = ConvertTo-Nfd $nfc
[void](New-TestFile $d $nfc 'existing')
$p = New-TestFile $d $nfd 'incoming'
$r = Invoke-Cli $p
Test-Case '같은 NFC 이름이 있으면 충돌로 보고하고 종료 코드 1을 반환한다' ($r.Code -eq 1 -and (Test-HasName $d $nfc) -and (Test-HasName $d $nfd)) $r.Output
Test-Case '충돌 시 기존 파일을 덮어쓰지 않는다' ((Read-TextOrNull (Join-Path $d $nfc)) -eq 'existing' -and (Read-TextOrNull $p) -eq 'incoming')

# ---------------------------------------------------------------------------
# 8. 폴더 자체
# ---------------------------------------------------------------------------
$d = New-CaseDir 'dir-self'
$folderNfc = '회의 자료'
$fp = New-TestDir $d (ConvertTo-Nfd $folderNfc)
$innerNfd = ConvertTo-Nfd '안건.txt'
[void](New-TestFile $fp $innerNfd)
$r = Invoke-Cli $fp
Test-Case '폴더를 지정하면 폴더 이름만 바꾸고 안의 항목은 그대로 둔다' ($r.Code -eq 0 -and (Test-HasName $d $folderNfc) -and (Test-HasName (Join-Path $d $folderNfc) $innerNfd)) $r.Output

# ---------------------------------------------------------------------------
# 9. --recursive (-r)
# ---------------------------------------------------------------------------
$d = New-CaseDir 'recursive'
$top = New-TestDir $d (ConvertTo-Nfd '상위')
$sub = New-TestDir $top (ConvertTo-Nfd '하위')
[void](New-TestFile $sub (ConvertTo-Nfd '파일.txt'))
[void](New-TestFile $top (ConvertTo-Nfd '첫째.txt'))
$r = Invoke-Cli -r $top
$ok = (Test-HasName $d '상위') -and (Test-HasName (Join-Path $d '상위') '하위') -and (Test-HasName (Join-Path $d '상위') '첫째.txt') -and (Test-HasName (Join-Path $d '상위\하위') '파일.txt')
Test-Case '-r은 폴더 자신과 모든 하위 항목을 정리한다' ($r.Code -eq 0 -and $ok) $r.Output

# ---------------------------------------------------------------------------
# 10. --children (-c), -c -r
# ---------------------------------------------------------------------------
function New-FamilyTree([string]$Dir) {
    $parent = New-TestDir $Dir (ConvertTo-Nfd '부모')
    [void](New-TestFile $parent (ConvertTo-Nfd '자식.txt'))
    $child = New-TestDir $parent (ConvertTo-Nfd '자식폴더')
    [void](New-TestFile $child (ConvertTo-Nfd '손자.txt'))
    $parent
}

$d = New-CaseDir 'children'
$parent = New-FamilyTree $d
$r = Invoke-Cli --children $parent
$ok = (Test-HasName $d (ConvertTo-Nfd '부모')) -and (Test-HasName $parent '자식.txt') -and (Test-HasName $parent '자식폴더') -and (Test-HasName (Join-Path $parent '자식폴더') (ConvertTo-Nfd '손자.txt'))
Test-Case '--children는 폴더 자신은 두고 직속 하위 항목만 정리한다' ($r.Code -eq 0 -and $ok) $r.Output

$d = New-CaseDir 'children-recursive'
$parent = New-FamilyTree $d
$r = Invoke-Cli -c -r $parent
$ok = (Test-HasName $d (ConvertTo-Nfd '부모')) -and (Test-HasName $parent '자식.txt') -and (Test-HasName $parent '자식폴더') -and (Test-HasName (Join-Path $parent '자식폴더') '손자.txt')
Test-Case '-c -r은 폴더 자신은 두고 모든 하위 항목을 정리한다' ($r.Code -eq 0 -and $ok) $r.Output

# ---------------------------------------------------------------------------
# 11. --dry-run (-n)
# ---------------------------------------------------------------------------
$d = New-CaseDir 'dry-run'
$nfd = ConvertTo-Nfd '미리보기.txt'
$p = New-TestFile $d $nfd
$logBefore = if (Test-Path -LiteralPath $logPath) { (Get-Item -LiteralPath $logPath).Length } else { 0 }
$r = Invoke-Cli --dry-run $p
$logAfter = if (Test-Path -LiteralPath $logPath) { (Get-Item -LiteralPath $logPath).Length } else { 0 }
Test-Case '--dry-run은 이름을 바꾸지 않는다' ($r.Code -eq 0 -and (Test-HasName $d $nfd) -and -not (Test-HasName $d '미리보기.txt')) $r.Output
Test-Case '--dry-run은 로그를 남기지 않는다' ($logBefore -eq $logAfter)

# ---------------------------------------------------------------------------
# 12. 여러 인자, 읽기 전용, 잠긴 파일, 없는 경로
# ---------------------------------------------------------------------------
$d = New-CaseDir 'multi'
$names = '가.txt', '나.txt', '다.txt'
$paths = foreach ($n in $names) { New-TestFile $d (ConvertTo-Nfd $n) }
$r = Invoke-Cli @paths
$ok = $true
foreach ($n in $names) { if (-not (Test-HasName $d $n)) { $ok = $false } }
Test-Case '여러 경로를 한 번에 처리한다' ($r.Code -eq 0 -and $ok) $r.Output

$d = New-CaseDir 'readonly'
$p = New-TestFile $d (ConvertTo-Nfd '읽기전용.txt')
(Get-Item -LiteralPath $p).IsReadOnly = $true
$r = Invoke-Cli $p
Test-Case '읽기 전용 파일도 이름을 바꾼다' ($r.Code -eq 0 -and (Test-HasName $d '읽기전용.txt')) $r.Output
$ro = Join-Path $d '읽기전용.txt'
if (Test-Path -LiteralPath $ro) { (Get-Item -LiteralPath $ro).IsReadOnly = $false }

$d = New-CaseDir 'locked'
$nfd = ConvertTo-Nfd '사용중.txt'
$p = New-TestFile $d $nfd
# 삭제 공유(FileShare.Delete) 없이 열어 두면 이름 변경이 거부됩니다.
$fs = [IO.File]::Open($p, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
try { $r = Invoke-Cli $p } finally { $fs.Dispose() }
Test-Case '다른 프로그램이 사용 중인 파일은 오류로 보고하고 원본을 유지한다' ($r.Code -eq 1 -and (Test-HasName $d $nfd)) $r.Output

$r = Invoke-Cli (Join-Path $work 'does-not-exist.txt')
Test-Case '없는 경로는 종료 코드 1을 반환한다' ($r.Code -eq 1) $r.Output

# ---------------------------------------------------------------------------
# 13. 변환 옵션 끄기
# ---------------------------------------------------------------------------
$d = New-CaseDir 'no-url'
$enc = [Uri]::EscapeDataString('인코딩') + '.txt'
$p = New-TestFile $d $enc
$r = Invoke-Cli --no-url $p
Test-Case '--no-url은 URL 인코딩을 풀지 않는다' ($r.Code -eq 0 -and (Test-HasName $d $enc)) $r.Output

$d = New-CaseDir 'no-nfc'
$nfd = ConvertTo-Nfd '분리.txt'
$p = New-TestFile $d $nfd
$r = Invoke-Cli --no-nfc $p
Test-Case '--no-nfc는 자소 분리를 합치지 않는다' ($r.Code -eq 0 -and (Test-HasName $d $nfd)) $r.Output

# ---------------------------------------------------------------------------
# 14. 사용법
# ---------------------------------------------------------------------------
$r = Invoke-Cli --help
Test-Case '--help는 종료 코드 0을 반환한다' ($r.Code -eq 0 -and $r.Output -match 'namefix') $r.Output
$r = Invoke-Cli --version
Test-Case '--version은 버전을 출력한다' ($r.Code -eq 0 -and $r.Output -match 'namefix \d+\.\d+\.\d+') $r.Output
$r = Invoke-Cli --no-such-option
Test-Case '알 수 없는 옵션은 종료 코드 2를 반환한다' ($r.Code -eq 2) $r.Output
$r = Invoke-Cli
Test-Case '경로가 없으면 종료 코드 2를 반환한다' ($r.Code -eq 2) $r.Output

# 탐색기가 "%V"로 드라이브 루트를 넘기면 "C:\"의 \" 가 따옴표 이스케이프로 해석되어 C:" 가 됩니다.
$outFile = Join-Path $work 'root-quote.out'
$proc = Start-Process -FilePath $cli -ArgumentList '--dry-run --children "C:\"' -NoNewWindow -Wait -PassThru -RedirectStandardOutput $outFile
Test-Case '드라이브 루트 인수의 따옴표 이스케이프("C:\")를 루트 경로로 복원한다' ($proc.ExitCode -eq 0) (Read-TextOrNull $outFile)

# ---------------------------------------------------------------------------
# 15. 로그
# ---------------------------------------------------------------------------
$logText = Read-TextOrNull $logPath
Test-Case '변경 내역을 로그에 기록한다' ($null -ne $logText -and $logText -match "`tRENAMED`t" -and $logText -match "`tCONFLICT`t" -and $logText -match "`tERROR`t")

# ---------------------------------------------------------------------------
# 16. 탐색기 다중 선택 모사: 파일마다 namefixw.exe --gui 를 거의 동시에 실행
# ---------------------------------------------------------------------------
$d = New-CaseDir 'gui-batch'
$names = 1..12 | ForEach-Object { "묶음 시험 $_.txt" }
$paths = foreach ($n in $names) { New-TestFile $d (ConvertTo-Nfd $n) }
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$procs = foreach ($p in $paths) { Start-Process -FilePath $gui -ArgumentList "--gui `"$p`"" -PassThru }
$procs | Wait-Process -Timeout 60
$ok = $true
foreach ($n in $names) { if (-not (Test-HasName $d $n)) { $ok = $false } }
$summaries = @(Get-SummaryLines)
Test-Case 'GUI 모드: 동시에 실행된 요청을 모두 처리한다' $ok ((Get-Names $d) -join ', ')
Test-Case 'GUI 모드: 동시에 실행된 요청을 한 프로세스가 모아 한 번에 처리한다' ($summaries.Count -eq 1 -and $summaries[0] -match 'inputs=12') ($summaries -join ' | ')
$codes = @($procs | ForEach-Object { $_.ExitCode })
Test-Case 'GUI 모드: 모든 프로세스가 종료 코드 0으로 끝난다' (@($codes | Where-Object { $_ -ne 0 }).Count -eq 0) ($codes -join ',')

# ---------------------------------------------------------------------------
# 17. 탐색기 빈 공간 메뉴 모사: --gui --children <폴더>
# ---------------------------------------------------------------------------
$d = New-CaseDir 'gui-background'
$names = '하나.txt', '둘.txt', '셋.txt'
foreach ($n in $names) { [void](New-TestFile $d (ConvertTo-Nfd $n)) }
$proc = Start-Process -FilePath $gui -ArgumentList "--gui --children `"$d`"" -Wait -PassThru
$ok = $true
foreach ($n in $names) { if (-not (Test-HasName $d $n)) { $ok = $false } }
Test-Case 'GUI 모드: 폴더 빈 공간 메뉴(--children)로 안의 항목을 정리한다' ($proc.ExitCode -eq 0 -and $ok) ((Get-Names $d) -join ', ')

$d = New-CaseDir 'gui-conflict'
$nfc = '겹침.txt'
[void](New-TestFile $d $nfc 'existing')
$p = New-TestFile $d (ConvertTo-Nfd $nfc) 'incoming'
$proc = Start-Process -FilePath $gui -ArgumentList "--gui `"$p`"" -Wait -PassThru
Test-Case 'GUI 모드: 충돌이 있으면 종료 코드 1을 반환하고 덮어쓰지 않는다' ($proc.ExitCode -eq 1 -and (Read-TextOrNull (Join-Path $d $nfc)) -eq 'existing') "exit=$($proc.ExitCode)"

# ---------------------------------------------------------------------------
# 18. 링크(정션) 폴더
# ---------------------------------------------------------------------------
$d = New-CaseDir 'junction-top'
$target = New-TestDir $d 'target'
$linkedNfd = ConvertTo-Nfd '링크 안.txt'
[void](New-TestFile $target $linkedNfd)
$link = Join-Path $d 'link'
[void](New-Item -ItemType Junction -Path $link -Target $target)
$r = Invoke-Cli -c $link
Test-Case '링크 폴더를 직접 지정해도(-c) 링크 대상 안의 항목은 바꾸지 않고 오류로 보고한다' ($r.Code -eq 1 -and (Test-HasName $target $linkedNfd)) $r.Output
$r = Invoke-Cli -r $link
Test-Case '링크 폴더를 직접 지정해도(-r) 링크 대상 안의 항목은 바꾸지 않고 오류로 보고한다' ($r.Code -eq 1 -and (Test-HasName $target $linkedNfd)) $r.Output

$d = New-CaseDir 'junction-nested'
$outside = New-TestDir $d 'outside'
[void](New-TestFile $outside $linkedNfd)
$tree = New-TestDir $d 'tree'
[void](New-Item -ItemType Junction -Path (Join-Path $tree (ConvertTo-Nfd '링크')) -Target $outside)
$r = Invoke-Cli -r $tree
Test-Case '-r 순회 중 만난 링크는 이름만 정리하고 안으로 들어가지 않는다' ($r.Code -eq 0 -and (Test-HasName $tree '링크') -and (Test-HasName $outside $linkedNfd)) $r.Output

# ---------------------------------------------------------------------------
# 19. 하드 링크: NFD 이름과 NFC 이름이 같은 파일을 가리키는 별개 항목
# ---------------------------------------------------------------------------
$d = New-CaseDir 'hardlink'
$nfc = '하드 링크.txt'
$nfd = ConvertTo-Nfd $nfc
$pNfc = New-TestFile $d $nfc 'same-file'
$pNfd = Join-Path $d $nfd
[void](New-Item -ItemType HardLink -Path $pNfd -Target $pNfc)
$r = Invoke-Cli --dry-run $pNfd
Test-Case '하드 링크로 이미 있는 이름은 미리 보기에서도 충돌로 보고한다' ($r.Code -eq 1) $r.Output
$r = Invoke-Cli $pNfd
Test-Case '하드 링크로 이미 있는 이름은 충돌로 보고하고 두 항목을 유지한다' ($r.Code -eq 1 -and (Test-HasName $d $nfc) -and (Test-HasName $d $nfd)) $r.Output

# 새 이름과 대소문자만 다른 별개 하드 링크(a x.txt)가 있는 경우
$d = New-CaseDir 'hardlink-case'
$pLower = New-TestFile $d 'a x.txt' 'same-file'
$pSource = Join-Path $d 'A%20x.txt'
[void](New-Item -ItemType HardLink -Path $pSource -Target $pLower)
$r = Invoke-Cli --dry-run $pSource
Test-Case '대소문자만 다른 하드 링크가 있으면 미리 보기에서도 충돌로 보고한다' ($r.Code -eq 1) $r.Output
$r = Invoke-Cli $pSource
Test-Case '대소문자만 다른 하드 링크가 있으면 충돌로 보고하고 두 항목을 유지한다' ($r.Code -eq 1 -and (Test-HasName $d 'a x.txt') -and (Test-HasName $d 'A%20x.txt')) $r.Output

# ---------------------------------------------------------------------------
# 20. 대소문자를 구분하는 폴더(지원 환경에서만)
# ---------------------------------------------------------------------------
$d = New-CaseDir 'case-sensitive'
$null = & fsutil.exe file setCaseSensitiveInfo $d enable 2>&1
if ($LASTEXITCODE -eq 0) {
    $p1 = New-TestFile $d 'A%20x.txt' 'upper'
    $p2 = New-TestFile $d 'a%20x.txt' 'lower'
    $r = Invoke-Cli $p1 $p2
    Test-Case '대소문자를 구분하는 폴더에서 대소문자만 다른 두 항목을 모두 처리한다' ($r.Code -eq 0 -and (Test-HasName $d 'A x.txt') -and (Test-HasName $d 'a x.txt')) $r.Output
}
else {
    Write-Host 'SKIP  대소문자 구분 폴더를 만들 수 없는 환경입니다(fsutil setCaseSensitiveInfo 실패).'
}

# ---------------------------------------------------------------------------
# 21. 다중 실행 집계의 견고성
# ---------------------------------------------------------------------------
# 같은 사용자·세션의 수집자 파이프 이름입니다(Collector.cs와 같은 규칙).
$pipeName = 'namefix-collector-' + [Security.Principal.WindowsIdentity]::GetCurrent().User.Value + '-' + (Get-Process -Id $PID).SessionId

# 21-1. 요청마다 자기 변환 옵션을 유지한다(수집자 옵션을 다른 요청에 적용하지 않음)
foreach ($order in 'default-first', 'no-url-first') {
    $d = New-CaseDir "gui-options-$order"
    $encDefault = [Uri]::EscapeDataString('기본 옵션') + '.txt'
    $encNoUrl = [Uri]::EscapeDataString('인코딩 유지') + '.txt'
    $pDefault = New-TestFile $d $encDefault
    $pNoUrl = New-TestFile $d $encNoUrl
    if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
    $env:NAMEFIX_COLLECT_MS = '1500'
    $argsDefault = "--gui `"$pDefault`""
    $argsNoUrl = "--gui --no-url `"$pNoUrl`""
    $first, $second = if ($order -eq 'default-first') { $argsDefault, $argsNoUrl } else { $argsNoUrl, $argsDefault }
    $pa = Start-Process -FilePath $gui -ArgumentList $first -PassThru
    Start-Sleep -Milliseconds 400
    $pb = Start-Process -FilePath $gui -ArgumentList $second -PassThru
    $pa, $pb | Wait-Process -Timeout 60
    Remove-Item Env:\NAMEFIX_COLLECT_MS
    $summaries = @(Get-SummaryLines)
    $ok = (Test-HasName $d '기본 옵션.txt') -and (Test-HasName $d $encNoUrl)
    Test-Case "GUI 모드: 집계된 요청마다 자기 옵션을 적용한다($order)" ($ok -and $summaries.Count -eq 1) ("names=" + ((Get-Names $d) -join ', ') + " summaries=" + $summaries.Count)
}

# 21-2. 연결만 하고 바로 끊거나 잘못된 메시지를 보내는 연결이 있어도 수집을 계속한다
$d = New-CaseDir 'gui-broken-client'
$pa = New-TestFile $d (ConvertTo-Nfd '견고성 가.txt')
$pb = New-TestFile $d (ConvertTo-Nfd '견고성 나.txt')
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$env:NAMEFIX_COLLECT_MS = '2000'
$procA = Start-Process -FilePath $gui -ArgumentList "--gui `"$pa`"" -PassThru
Start-Sleep -Milliseconds 400
foreach ($kind in 'hangup', 'garbage', 'hangup') {
    $client = [IO.Pipes.NamedPipeClientStream]::new('.', $pipeName, [IO.Pipes.PipeDirection]::InOut)
    try {
        $client.Connect(3000)
        if ($kind -eq 'garbage') { $client.Write([byte[]](0xFF, 0xFF, 0xFF, 0x7F, 1, 2, 3), 0, 7); $client.Flush() }
    }
    catch { Write-Host "      (연결 시도 실패: $($_.Exception.Message))" }
    finally { $client.Dispose() }
}
$procB = Start-Process -FilePath $gui -ArgumentList "--gui `"$pb`"" -PassThru
$procA, $procB | Wait-Process -Timeout 60
Remove-Item Env:\NAMEFIX_COLLECT_MS
$summaries = @(Get-SummaryLines)
$ok = (Test-HasName $d '견고성 가.txt') -and (Test-HasName $d '견고성 나.txt')
Test-Case 'GUI 모드: 비정상 연결이 있어도 수집자가 이미 받은 요청과 이후 요청을 모두 처리한다' ($ok -and $summaries.Count -eq 1 -and $procA.ExitCode -eq 0) ("names=" + ((Get-Names $d) -join ', ') + " summaries=" + $summaries.Count + " exitA=" + $procA.ExitCode)

# 21-3. ACK가 보내는 쪽의 대기 시간(3초)보다 늦게 오면 같은 요청 ID로 다시 보내고, 수집자는 한 번만 넣는다
$d = New-CaseDir 'gui-ack-timeout'
$pa = New-TestFile $d (ConvertTo-Nfd '재전송 가.txt')
$pb = New-TestFile $d (ConvertTo-Nfd '재전송 나.txt')
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$env:NAMEFIX_COLLECT_MS = '1500'
$env:NAMEFIX_TEST_ACK_DELAY_MS = '4000'
$procA = Start-Process -FilePath $gui -ArgumentList "--gui `"$pa`"" -PassThru
Start-Sleep -Milliseconds 400
$procB = Start-Process -FilePath $gui -ArgumentList "--gui `"$pb`"" -PassThru
$procA, $procB | Wait-Process -Timeout 90
Remove-Item Env:\NAMEFIX_COLLECT_MS, Env:\NAMEFIX_TEST_ACK_DELAY_MS
$summaries = @(Get-SummaryLines)
$logText = Read-TextOrNull $logPath
$ok = (Test-HasName $d '재전송 가.txt') -and (Test-HasName $d '재전송 나.txt')
Test-Case 'GUI 모드: ACK 시간 초과 후 재전송한 요청을 수집자가 한 번만 넣는다' ($ok -and $summaries.Count -eq 1 -and $summaries[0] -match 'inputs=2' -and $logText -notmatch "`tERROR`t") ($summaries -join ' | ')

# 21-4. 멱등성과 "이미 정리됨" 판정
$d = New-CaseDir 'idempotent'
$deep = '%25252541' + '.txt'     # 4단계 인코딩: %25252541 → %252541 → %2541 → %41 → A
$p = New-TestFile $d $deep
$r1 = Invoke-Cli $p
$r2 = Invoke-Cli -c $d
Test-Case '여러 단계로 인코딩된 이름을 한 번에 끝까지 풀어 다시 실행해도 바뀌지 않는다' ($r1.Code -eq 0 -and (Test-HasName $d 'A.txt') -and $r2.Code -eq 0 -and $r2.Output -match '변경 0') ($r1.Output + $r2.Output)

# 처음부터 없는 경로는, 이름이 대응하는 다른 항목이 있어도 "이미 정리됨"으로 대체하지 않습니다(원본 식별자 없음).
$d = New-CaseDir 'unrelated-target'
$unrelated = New-TestDir $d 'A'
[void](New-TestFile $unrelated '%42.txt')
$r = Invoke-Cli -r (Join-Path $d '%41')
Test-Case '처음부터 없는 경로는 이름이 대응하는 다른 폴더로 대체하지 않고 오류로 보고한다' ($r.Code -eq 1 -and (Test-HasName $unrelated '%42.txt') -and -not (Test-HasName $unrelated 'B.txt')) $r.Output

# 21-5. 수집자가 요청을 받은 뒤 처리 전에 비정상 종료하면, 보낸 쪽이 자기 요청을 직접 처리한다(처리 최소 한 번)
$d = New-CaseDir 'gui-crash-before'
$pa = New-TestFile $d (ConvertTo-Nfd '종료 전 가.txt')
$pb = New-TestFile $d (ConvertTo-Nfd '종료 전 나.txt')
$marker = Join-Path $work ('crash-before-' + [guid]::NewGuid().ToString('N'))
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$env:NAMEFIX_COLLECT_MS = '1500'
$env:NAMEFIX_TEST_CRASH_POINT = 'before-process'
$env:NAMEFIX_TEST_CRASH_MARKER = $marker
$procA = Start-Process -FilePath $gui -ArgumentList "--gui `"$pa`"" -PassThru
Start-Sleep -Milliseconds 400
$procB = Start-Process -FilePath $gui -ArgumentList "--gui `"$pb`"" -PassThru
$procA, $procB | Wait-Process -Timeout 90
Remove-Item Env:\NAMEFIX_COLLECT_MS, Env:\NAMEFIX_TEST_CRASH_POINT, Env:\NAMEFIX_TEST_CRASH_MARKER
$crashed = Test-Path -LiteralPath $marker
Test-Case 'GUI 모드: 처리 전에 수집자가 종료되면 보낸 쪽이 자기 요청을 직접 처리한다' ($crashed -and $procB.ExitCode -eq 0 -and (Test-HasName $d '종료 전 나.txt') -and (Test-HasName $d (ConvertTo-Nfd '종료 전 가.txt'))) ("crashed=$crashed exitB=$($procB.ExitCode) names=" + ((Get-Names $d) -join ', '))

# 21-6. 수집자가 처리를 마친 뒤 완료를 알리기 전에 비정상 종료하면, 보낸 쪽이 다시 처리해도 오류 없이 "이미 정리됨"으로 끝난다
$d = New-CaseDir 'gui-crash-after'
$pa = New-TestFile $d (ConvertTo-Nfd '종료 후 가.txt')
$pb = New-TestFile $d (ConvertTo-Nfd '종료 후 나.txt')
$marker = Join-Path $work ('crash-after-' + [guid]::NewGuid().ToString('N'))
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$env:NAMEFIX_COLLECT_MS = '1500'
$env:NAMEFIX_TEST_CRASH_POINT = 'after-process'
$env:NAMEFIX_TEST_CRASH_MARKER = $marker
$procA = Start-Process -FilePath $gui -ArgumentList "--gui `"$pa`"" -PassThru
Start-Sleep -Milliseconds 400
$procB = Start-Process -FilePath $gui -ArgumentList "--gui `"$pb`"" -PassThru
$procA, $procB | Wait-Process -Timeout 90
Remove-Item Env:\NAMEFIX_COLLECT_MS, Env:\NAMEFIX_TEST_CRASH_POINT, Env:\NAMEFIX_TEST_CRASH_MARKER
$crashed = Test-Path -LiteralPath $marker
$logText = Read-TextOrNull $logPath
$ok = (Test-HasName $d '종료 후 가.txt') -and (Test-HasName $d '종료 후 나.txt')
Test-Case 'GUI 모드: 처리 후 완료 알림 전에 수집자가 종료되면 다시 처리해도 오류 없이 이미 정리됨으로 끝난다' ($crashed -and $ok -and $procB.ExitCode -eq 0 -and $null -ne $logText -and $logText -match 'already=1' -and $logText -notmatch "`tERROR`t") ("crashed=$crashed exitB=$($procB.ExitCode) log=" + $logText)

# 21-7. 같은 트리를 동시에 두 번 실행해도 실행 잠금으로 직렬화되어 오류가 나지 않는다
$d = New-CaseDir 'concurrent-duplicate'
$tree = New-TestDir $d (ConvertTo-Nfd '동시 실행')
foreach ($i in 1..400) { [void](New-TestFile $tree (ConvertTo-Nfd ('동시 파일 {0:D3}.txt' -f $i))) }
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$procs = foreach ($k in 1..2) { Start-Process -FilePath $cli -ArgumentList "--quiet -r `"$tree`"" -WindowStyle Hidden -PassThru }
$procs | Wait-Process -Timeout 120
$codes = @($procs | ForEach-Object { $_.ExitCode })
$treeNfc = Join-Path $d '동시 실행'
$remainingNfd = @(if (Test-Path -LiteralPath $treeNfc) { Get-Names $treeNfc | Where-Object { -not $_.IsNormalized([Text.NormalizationForm]::FormC) } }).Count
$logText = Read-TextOrNull $logPath
Test-Case '같은 트리를 동시에 두 번 실행해도 두 실행 모두 오류 없이 끝나고 모두 정리된다' ((@($codes | Where-Object { $_ -ne 0 }).Count -eq 0) -and (Test-Path -LiteralPath $treeNfc) -and $remainingNfd -eq 0 -and $logText -notmatch "`tERROR`t") ("codes=" + ($codes -join ',') + " remainingNfd=$remainingNfd")

# 21-8. 절대 경로로 확정할 수 없는 경로는 수집자에게 보내지 않고 그 프로세스에서 오류로 보고한다
$d = New-CaseDir 'gui-unresolvable'
$pa = New-TestFile $d (ConvertTo-Nfd '정상 요청.txt')
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$env:NAMEFIX_COLLECT_MS = '1500'
$procA = Start-Process -FilePath $gui -ArgumentList "--gui `"$pa`"" -PassThru
Start-Sleep -Milliseconds 400
$procB = Start-Process -FilePath $gui -ArgumentList '--gui "bad|name.txt"' -PassThru
$procA, $procB | Wait-Process -Timeout 60
Remove-Item Env:\NAMEFIX_COLLECT_MS
$summaries = @(Get-SummaryLines)
$fromA = @($summaries | Where-Object { $_ -match "pid=$($procA.Id)`t" })
Test-Case 'GUI 모드: 해석할 수 없는 경로는 보내지 않고 자기 프로세스에서 오류(종료 코드 1)로 보고한다' ($procB.ExitCode -eq 1 -and $fromA.Count -eq 1 -and $fromA[0] -match 'inputs=1' -and (Test-HasName $d '정상 요청.txt')) ("exitB=$($procB.ExitCode) summaries=" + ($summaries -join ' | '))

# 21-8a. 요청 뒤 원본이 정리되고 같은 경로에 새 항목이 생기면, 새 항목은 건드리지 않고 원본을 현재 경로에서 처리한다
#        NAMEFIX_TEST_PAUSE_MS로 요청 확정 뒤 처리 전에 멈추게 하고, 그동안 테스트가 경로를 바꿉니다.
$d = New-CaseDir 'replaced-path'
$orig = New-TestDir $d '%41'
[void](New-TestFile $orig '%43.txt' 'original-child')
$env:NAMEFIX_TEST_PAUSE_MS = '3000'
$proc = Start-Process -FilePath $cli -ArgumentList "--quiet -r `"$orig`"" -WindowStyle Hidden -PassThru
Remove-Item Env:\NAMEFIX_TEST_PAUSE_MS
Start-Sleep -Milliseconds 1000
# 일시 정지 지점이 없는 실행 파일이면 이미 처리가 끝나 경로가 없을 수 있으므로, 이동 실패는 테스트 실패로 기록합니다.
$moved = $true
try {
    [IO.Directory]::Move($orig, (Join-Path $d 'A'))       # 다른 실행이 원본 폴더 이름을 먼저 정리한 상황
    $newDir = New-TestDir $d '%41'                        # 같은 경로에 새로 생긴 다른 폴더
    [void](New-TestFile $newDir '%42.txt' 'new-child')
}
catch { $moved = $false; $newDir = Join-Path $d '%41' }
$exited = $proc.WaitForExit(30000)
$ok = $moved -and (Test-HasName $newDir '%42.txt') -and (Test-HasName $d '%41') -and (Test-HasName (Join-Path $d 'A') 'C.txt')
Test-Case '요청 경로에 새로 생긴 다른 항목은 처리하지 않고, 원본을 식별자로 찾아 현재 경로에서 처리한다' ($exited -and $proc.ExitCode -eq 0 -and $ok) ("exit=$($proc.ExitCode) new=" + ((Get-Names $newDir) -join ',') + " A=" + ((Get-Names (Join-Path $d 'A')) -join ','))

# 21-8b. 요청 뒤 상위 폴더 이름까지 바뀌어도 식별자로 원본의 현재 경로를 찾아 처리한다
$d = New-CaseDir 'ancestor-renamed'
$outer = New-TestDir $d '%41'
$inner = New-TestDir $outer '%42'
[void](New-TestFile $inner '%43.txt')
$env:NAMEFIX_TEST_PAUSE_MS = '3000'
$proc = Start-Process -FilePath $cli -ArgumentList "--quiet -r `"$inner`"" -WindowStyle Hidden -PassThru
Remove-Item Env:\NAMEFIX_TEST_PAUSE_MS
Start-Sleep -Milliseconds 1000
$moved = $true
try {
    [IO.Directory]::Move($inner, (Join-Path $outer 'B'))  # 다른 실행이 요청 폴더와
    [IO.Directory]::Move($outer, (Join-Path $d 'A'))      # 그 상위 폴더 이름을 먼저 정리한 상황
}
catch { $moved = $false }
$exited = $proc.WaitForExit(30000)
$abPath = Join-Path $d 'A\B'
Test-Case '상위 폴더 이름이 바뀐 요청도 식별자로 현재 경로를 찾아 오류 없이 처리한다' ($moved -and $exited -and $proc.ExitCode -eq 0 -and (Test-HasName $abPath 'C.txt')) ("moved=$moved exit=$($proc.ExitCode) names=" + $(if (Test-Path -LiteralPath $abPath) { (Get-Names $abPath) -join ',' } else { '<없음>' }))

# 21-8d. 요청 당시 없던 경로는 처리 전에 같은 경로에 항목이 생겨도 처리하지 않는다
$d = New-CaseDir 'created-after-request'
$missing = Join-Path $d '%41'
$env:NAMEFIX_TEST_PAUSE_MS = '3000'
$proc = Start-Process -FilePath $cli -ArgumentList "--quiet -r `"$missing`"" -WindowStyle Hidden -PassThru
Remove-Item Env:\NAMEFIX_TEST_PAUSE_MS
Start-Sleep -Milliseconds 1000
$created = New-TestDir $d '%41'
[void](New-TestFile $created '%42.txt' 'created-later')
$exited = $proc.WaitForExit(30000)
Test-Case '요청 당시 없던 경로에 나중에 생긴 항목은 처리하지 않고 오류로 보고한다' ($exited -and $proc.ExitCode -eq 1 -and (Test-HasName $d '%41') -and (Test-HasName $created '%42.txt')) ("exit=$($proc.ExitCode) names=" + ((Get-Names $created) -join ','))

# 21-8e. 식별자로 여는 기능(OpenFileById)을 쓸 수 없을 때(SMB 등)도, 정리된 이름을 따라가며 식별자가 같은 원본을 찾는다
#        NAMEFIX_TEST_DISABLE_OPEN_BY_ID=1로 OpenFileById를 끄고 로컬 NTFS에서 대체 탐색을 확인합니다.
function Test-RecoverWithoutOpenById([string]$Base, [string]$Label) {
    $case = Join-Path $Base ('recover-' + [guid]::NewGuid().ToString('N').Substring(0, 6))
    [void][IO.Directory]::CreateDirectory($case)
    $outer = New-TestDir $case '%41'
    $inner = New-TestDir $outer '%42'
    [void](New-TestFile $inner '%43.txt' 'child')
    $file = New-TestFile $case '%44.txt' 'file'
    $env:NAMEFIX_TEST_PAUSE_MS = '3000'
    $env:NAMEFIX_TEST_DISABLE_OPEN_BY_ID = '1'
    $pDir = Start-Process -FilePath $cli -ArgumentList "--quiet -r `"$inner`"" -WindowStyle Hidden -PassThru
    $pFile = Start-Process -FilePath $cli -ArgumentList "--quiet `"$file`"" -WindowStyle Hidden -PassThru
    Remove-Item Env:\NAMEFIX_TEST_PAUSE_MS, Env:\NAMEFIX_TEST_DISABLE_OPEN_BY_ID
    Start-Sleep -Milliseconds 1000
    $moved = $true
    try {
        [IO.Directory]::Move($inner, (Join-Path $outer 'B'))   # 요청 폴더와
        [IO.Directory]::Move($outer, (Join-Path $case 'A'))    # 상위 폴더가 먼저 정리되고
        [IO.File]::Move($file, (Join-Path $case 'D.txt'))      # 요청 파일도 먼저 정리된 상황
    }
    catch { $moved = $false }
    $exitedDir = $pDir.WaitForExit(30000)
    $exitedFile = $pFile.WaitForExit(30000)
    $ok = $moved -and $exitedDir -and $exitedFile -and $pDir.ExitCode -eq 0 -and $pFile.ExitCode -eq 0 -and
        (Test-HasName (Join-Path $case 'A\B') 'C.txt') -and (Test-HasName $case 'D.txt')
    Test-Case "식별자로 열 수 없을 때도 정리된 이름을 따라가 원본을 찾아 처리한다($Label)" $ok ("moved=$moved dir=$($pDir.ExitCode) file=$($pFile.ExitCode) A\B=" + ((Get-Names (Join-Path $case 'A\B')) -join ','))
    try { Remove-Item -LiteralPath $case -Recurse -Force } catch { }
}
Test-RecoverWithoutOpenById (New-CaseDir 'recover-local') '로컬 NTFS'

# 실제 SMB 공유(로컬 관리 공유)에서도 확인합니다. 공유에 접근할 수 없는 환경에서는 건너뜁니다.
$smbBase = '\\localhost\' + $work.Substring(0, 1) + '$' + $work.Substring(2)
if (Test-Path -LiteralPath $smbBase) {
    Test-RecoverWithoutOpenById $smbBase 'SMB 공유'

    # SMB에서는 OpenFileById 없이 대체 탐색만으로 복구되는지(테스트 지점 없이) 확인합니다.
    $case = Join-Path $smbBase ('smb-native-' + [guid]::NewGuid().ToString('N').Substring(0, 6))
    [void][IO.Directory]::CreateDirectory($case)
    $file = New-TestFile $case '%45.txt' 'smb'
    $env:NAMEFIX_TEST_PAUSE_MS = '3000'
    $pSmb = Start-Process -FilePath $cli -ArgumentList "--quiet `"$file`"" -WindowStyle Hidden -PassThru
    Remove-Item Env:\NAMEFIX_TEST_PAUSE_MS
    Start-Sleep -Milliseconds 1000
    [IO.File]::Move($file, (Join-Path $case 'E.txt'))
    $exited = $pSmb.WaitForExit(30000)
    Test-Case 'SMB 공유에서 먼저 정리된 요청 파일을 오류 없이 이미 정리됨으로 처리한다' ($exited -and $pSmb.ExitCode -eq 0 -and (Test-HasName $case 'E.txt')) "exit=$($pSmb.ExitCode)"
}
else {
    Write-Host "SKIP  SMB 공유($smbBase)에 접근할 수 없어 SMB 복구 시험을 건너뜁니다."
}

# 21-8f. 같은 항목을 다른 경로 표기(대소문자, 정션 별칭)로 두 번 지정해도 한 번만 처리하고 허위 충돌·오류가 없다
$d = New-CaseDir 'alias-dup'
[void](New-TestFile $d 'A%20x.txt' 'one')
$dUpper = Join-Path $work 'ALIAS-DUP'
$rDry = Invoke-Cli -n (Join-Path $d 'A%20x.txt') (Join-Path $dUpper 'A%20x.txt')
$rRun = Invoke-Cli (Join-Path $d 'A%20x.txt') (Join-Path $dUpper 'A%20x.txt')
Test-Case '같은 파일을 대소문자가 다른 경로로 두 번 지정해도 미리 보기·실행 모두 한 번만 처리한다' ($rDry.Code -eq 0 -and $rDry.Output -match '예정 1' -and $rRun.Code -eq 0 -and (Test-HasName $d 'A x.txt')) ($rDry.Output + ' | ' + $rRun.Output)

$d = New-CaseDir 'alias-dup-junction'
$real = New-TestDir $d 'real'
[void](New-TestFile $real 'B%20y.txt' 'two')
$alias = Join-Path $d 'alias'
[void](New-Item -ItemType Junction -Path $alias -Target $real)
$rRun = Invoke-Cli (Join-Path $real 'B%20y.txt') (Join-Path $alias 'B%20y.txt')
Test-Case '같은 파일을 정션 별칭 경로로 함께 지정해도 한 번만 처리하고 오류가 없다' ($rRun.Code -eq 0 -and (Test-HasName $real 'B y.txt')) $rRun.Output

# 21-8g. 대체 탐색도 하드 링크가 여러 개인 파일은 원본으로 인정하지 않는다(같은 파일 ID의 다른 디렉터리 항목)
$d = New-CaseDir 'fallback-hardlink'
$dirOrig = New-TestDir $d '%41'
$dirOther = New-TestDir $d 'A'
$requested = New-TestFile $dirOrig '%42.txt' 'shared'
$otherLink = Join-Path $dirOther '%42.txt'
[void](New-Item -ItemType HardLink -Path $otherLink -Target $requested)
$env:NAMEFIX_TEST_PAUSE_MS = '3000'
$proc = Start-Process -FilePath $cli -ArgumentList "--quiet `"$requested`"" -WindowStyle Hidden -PassThru
Remove-Item Env:\NAMEFIX_TEST_PAUSE_MS
Start-Sleep -Milliseconds 1000
$moved = $true
try { [IO.File]::Move($requested, (Join-Path $dirOrig 'keep.txt')) } catch { $moved = $false }   # 요청한 항목만 다른 이름으로 옮김
$exited = $proc.WaitForExit(30000)
Test-Case '대체 탐색은 하드 링크가 여러 개인 파일을 원본으로 인정하지 않아 다른 링크를 바꾸지 않는다' ($moved -and $exited -and $proc.ExitCode -eq 1 -and (Test-HasName $dirOther '%42.txt') -and -not (Test-HasName $dirOther 'B.txt')) ("moved=$moved exit=$($proc.ExitCode) A=" + ((Get-Names $dirOther) -join ','))

# 21-8h. 정션 별칭으로 지정한 하위 항목과 실제 경로로 지정한 부모를 함께 처리해도 실제 경로 깊이 순서로 처리한다
$d = New-CaseDir 'junction-depth'
$deep = New-TestDir $d 'w\deep'
$parentReq = New-TestDir $deep '%41'
[void](New-TestFile $parentReq '%42.txt')
$shortAlias = Join-Path $d 'j'
[void](New-Item -ItemType Junction -Path $shortAlias -Target $deep)
$r = Invoke-Cli $parentReq (Join-Path $shortAlias '%41\%42.txt')
Test-Case '정션 별칭 경로의 하위 항목을 실제 경로 기준으로 부모보다 먼저 처리한다' ($r.Code -eq 0 -and (Test-HasName (Join-Path $deep 'A') 'B.txt')) ($r.Output + ' | ' + ((Get-Names (Join-Path $deep 'A')) -join ','))

# 21-8j. 경로 중간의 정션 자체가 처리 대상이어도, 그 정션을 거치는 하위 요청을 먼저 처리한다
$d = New-CaseDir 'junction-in-path'
$realTarget = New-TestDir $d 'real'
[void](New-TestFile $realTarget '%42.txt')
$deepParent = New-TestDir $d 'w\deep'
$junctionReq = Join-Path $deepParent '%41'
[void](New-Item -ItemType Junction -Path $junctionReq -Target $realTarget)
$r = Invoke-Cli $junctionReq (Join-Path $junctionReq '%42.txt')
Test-Case '경로 중간의 정션이 처리 대상이어도 그 정션을 거치는 하위 요청을 먼저 처리한다' ($r.Code -eq 0 -and (Test-HasName $realTarget 'B.txt') -and (Test-HasName $deepParent 'A')) ($r.Output + ' | real=' + ((Get-Names $realTarget) -join ',') + ' deep=' + ((Get-Names $deepParent) -join ','))

# 21-8l. 정션이 처리 대상 폴더 자체를 가리킬 때, 그 정션을 거치는 하위 요청을 먼저 처리한다(실제 경로의 상위 관계)
$d = New-CaseDir 'junction-to-target'
$deepParent = New-TestDir $d 'w\deep'
$targetDir = New-TestDir $deepParent '%41'
[void](New-TestFile $targetDir '%42.txt')
$toTarget = Join-Path $d 'j'
[void](New-Item -ItemType Junction -Path $toTarget -Target $targetDir)
$rDry = Invoke-Cli -n $targetDir (Join-Path $toTarget '%42.txt')
$r = Invoke-Cli $targetDir (Join-Path $toTarget '%42.txt')
Test-Case '처리 대상 폴더 자체를 가리키는 정션을 거친 하위 요청을 그 폴더보다 먼저 처리한다' ($rDry.Code -eq 0 -and $r.Code -eq 0 -and (Test-HasName (Join-Path $deepParent 'A') 'B.txt')) ($r.Output + ' | A=' + ((Get-Names (Join-Path $deepParent 'A')) -join ','))

# 21-8m. 정션이 연쇄로 이어져 중간 경유지가 처리 대상이어도 하위 요청을 정상 처리한다
#        j → w\deep\%41\hop(정션) → r. 파일 요청은 j를 거치고, 중간 경유지 %41이 처리 대상입니다.
$d = New-CaseDir 'junction-chain'
$finalDir = New-TestDir $d 'r'
[void](New-TestFile $finalDir '%42.txt')
$deepParent = New-TestDir $d 'w\deep'
$viaDir = New-TestDir $deepParent '%41'
[void](New-Item -ItemType Junction -Path (Join-Path $viaDir 'hop') -Target $finalDir)
$chainStart = Join-Path $d 'j'
[void](New-Item -ItemType Junction -Path $chainStart -Target (Join-Path $viaDir 'hop'))
$rDry = Invoke-Cli -n $viaDir (Join-Path $chainStart '%42.txt')
$r = Invoke-Cli $viaDir (Join-Path $chainStart '%42.txt')
Test-Case '연쇄 정션의 중간 경유지가 처리 대상이어도 하위 요청을 정상 처리한다' ($rDry.Code -eq 0 -and $r.Code -eq 0 -and (Test-HasName $finalDir 'B.txt') -and (Test-HasName $deepParent 'A')) ($r.Output + ' | r=' + ((Get-Names $finalDir) -join ','))

# 21-8k. 드라이브 루트 바로 아래 폴더와 그 하위 항목을 함께 처리한다(subst로 만든 임시 드라이브에서 시험)
$d = New-CaseDir 'drive-root'
$freeLetter = [char[]]('ZYXWVUTSRQPONMLKJIHG') | Where-Object { -not (Test-Path -LiteralPath "$($_):\") } | Select-Object -First 1
if ($null -ne $freeLetter) {
    $drive = "$($freeLetter):"
    & subst.exe $drive $d | Out-Null
    try {
        if (Test-Path -LiteralPath "$drive\") {
            $rootDir = New-TestDir "$drive\" '%41'
            [void](New-TestFile $rootDir '%42.txt')
            $rDry = Invoke-Cli -n "$drive\%41" "$drive\%41\%42.txt"
            $r = Invoke-Cli "$drive\%41" "$drive\%41\%42.txt"
            Test-Case '드라이브 루트 바로 아래 폴더와 그 하위 파일을 함께 지정해도 하위부터 처리한다' ($rDry.Code -eq 0 -and $r.Code -eq 0 -and (Test-HasName "$drive\A" 'B.txt')) ($rDry.Output + ' | ' + $r.Output)

            $rootDir2 = New-TestDir "$drive\" '%43'
            [void](New-TestFile $rootDir2 '%44.txt')
            $r = Invoke-Cli -c -r "$drive\"
            Test-Case '드라이브 루트를 -c -r로 순회해도 하위부터 처리한다' ($r.Code -eq 0 -and (Test-HasName "$drive\C" 'D.txt')) $r.Output
        }
        else {
            Write-Host "SKIP  subst 드라이브($drive)를 만들 수 없어 드라이브 루트 시험을 건너뜁니다."
        }
    }
    finally { & subst.exe $drive /d | Out-Null }
}
else {
    Write-Host 'SKIP  사용할 수 있는 드라이브 문자가 없어 드라이브 루트 시험을 건너뜁니다.'
}

# 21-8i. 입력한 경로 표기의 대소문자가 아니라 디스크에 저장된 이름을 정리한다
$d = New-CaseDir 'stored-name-case'
[void](New-TestFile $d 'a%20x.txt' 'stored')
$r = Invoke-Cli (Join-Path $d 'A%20X.TXT')
Test-Case '입력 경로의 대소문자가 달라도 저장된 이름을 기준으로 정리해 대소문자를 바꾸지 않는다' ($r.Code -eq 0 -and (Test-HasName $d 'a x.txt') -and -not (Test-HasName $d 'A X.TXT')) ($r.Output + ' | ' + ((Get-Names $d) -join ','))

# 21-8c. 넘겨준 요청의 결과(충돌·오류 여부)가 보낸 프로세스의 종료 코드로 돌아온다
foreach ($order in 'normal-collects', 'conflict-collects') {
    $d = New-CaseDir "gui-forwarded-exit-$order"
    $pNormal = New-TestFile $d (ConvertTo-Nfd '정상 전달.txt')
    [void](New-TestFile $d '충돌 전달.txt' 'existing')
    $pConflict = New-TestFile $d (ConvertTo-Nfd '충돌 전달.txt') 'incoming'
    if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
    $env:NAMEFIX_COLLECT_MS = '1500'
    $first, $second = if ($order -eq 'normal-collects') { $pNormal, $pConflict } else { $pConflict, $pNormal }
    $p1 = Start-Process -FilePath $gui -ArgumentList "--gui `"$first`"" -PassThru
    Start-Sleep -Milliseconds 400
    $p2 = Start-Process -FilePath $gui -ArgumentList "--gui `"$second`"" -PassThru
    $p1, $p2 | Wait-Process -Timeout 60
    Remove-Item Env:\NAMEFIX_COLLECT_MS
    $exitNormal = if ($order -eq 'normal-collects') { $p1.ExitCode } else { $p2.ExitCode }
    $exitConflict = if ($order -eq 'normal-collects') { $p2.ExitCode } else { $p1.ExitCode }
    $summaries = @(Get-SummaryLines)
    Test-Case "GUI 모드: 각 프로세스의 종료 코드는 자기 요청의 결과를 따른다($order)" ($exitNormal -eq 0 -and $exitConflict -eq 1 -and $summaries.Count -eq 1) ("normal=$exitNormal conflict=$exitConflict summaries=" + $summaries.Count)
}

# 21-9. 상대 경로는 각 프로세스의 작업 폴더 기준으로 확정한 뒤 보낸다
$d = New-CaseDir 'gui-relative'
$dirA = New-TestDir $d 'A'
$dirB = New-TestDir $d 'B'
$rel = [Uri]::EscapeDataString('상대 경로') + '.txt'
[void](New-TestFile $dirA $rel 'from-A')
[void](New-TestFile $dirB $rel 'from-B')
if (Test-Path -LiteralPath $logPath) { Remove-Item -LiteralPath $logPath }
$env:NAMEFIX_COLLECT_MS = '1500'
$procA = Start-Process -FilePath $gui -ArgumentList "--gui `"$rel`"" -WorkingDirectory $dirA -PassThru
Start-Sleep -Milliseconds 400
$procB = Start-Process -FilePath $gui -ArgumentList "--gui `"$rel`"" -WorkingDirectory $dirB -PassThru
$procA, $procB | Wait-Process -Timeout 60
Remove-Item Env:\NAMEFIX_COLLECT_MS
$summaries = @(Get-SummaryLines)
$ok = (Read-TextOrNull (Join-Path $dirA '상대 경로.txt')) -eq 'from-A' -and (Read-TextOrNull (Join-Path $dirB '상대 경로.txt')) -eq 'from-B'
Test-Case 'GUI 모드: 작업 폴더가 다른 프로세스의 상대 경로를 각자의 폴더 기준으로 처리한다' ($ok -and $summaries.Count -eq 1) ("A=" + ((Get-Names $dirA) -join ',') + " B=" + ((Get-Names $dirB) -join ',') + " summaries=" + $summaries.Count)

# 21-10. 연결만 받고 읽지 않는 수집자가 있어도 보내는 쪽이 멈추지 않는다(쓰기 시간 제한)
#        테스트가 수집자 뮤텍스를 쥐고, 연결은 받지만 읽지 않는 파이프 서버를 흉내 냅니다.
$d = New-CaseDir 'gui-stalled-collector'
# 요청 본문이 파이프 버퍼(1KB)보다 훨씬 커야 쓰기가 막힙니다(80개 × 약 300바이트).
$names = 1..80 | ForEach-Object { '멈춘 수집자 {0:D3} {1}.txt' -f $_, ('긴이름' * 10) }
$paths = foreach ($n in $names) { New-TestFile $d (ConvertTo-Nfd $n) }
$fakeMutex = [Threading.Mutex]::new($false, 'Local\' + $pipeName)
$fakeServer = $null
$stalledStart = [DateTime]::UtcNow
try {
    if (-not $fakeMutex.WaitOne(0)) { throw '수집자 뮤텍스를 얻지 못했습니다(다른 namefix가 실행 중).' }
    $fakeServer = [IO.Pipes.NamedPipeServerStream]::new($pipeName, [IO.Pipes.PipeDirection]::InOut, 1,
        [IO.Pipes.PipeTransmissionMode]::Byte, [IO.Pipes.PipeOptions]::Asynchronous, 1024, 1024)
    [void]$fakeServer.BeginWaitForConnection($null, $null)
    $argLine = '--gui ' + (($paths | ForEach-Object { "`"$_`"" }) -join ' ')
    $proc = Start-Process -FilePath $gui -ArgumentList $argLine -PassThru
    $exited = $proc.WaitForExit(60000)
}
finally {
    if ($null -ne $fakeServer) { $fakeServer.Dispose() }
    try { $fakeMutex.ReleaseMutex() } catch { }
    $fakeMutex.Dispose()
}
$elapsed = ([DateTime]::UtcNow - $stalledStart).TotalSeconds
$ok = $true
foreach ($n in $names) { if (-not (Test-HasName $d $n)) { $ok = $false } }
Test-Case 'GUI 모드: 읽지 않는 수집자에게 막혀도 제한 시간 뒤 자기 요청을 직접 처리한다' ($exited -and $proc.ExitCode -eq 0 -and $ok) ("exited=$exited elapsed={0:N1}s" -f $elapsed)

# ---------------------------------------------------------------------------
# 22. 로그 교체: 여러 프로세스가 동시에 교체 조건을 만나도 이전 기록을 잃지 않는다
# ---------------------------------------------------------------------------
$d = New-CaseDir 'log-rotate'
$rotateLog = Join-Path $work 'rotate.log'
$marker = 'ROTATE-MARKER-' + [guid]::NewGuid().ToString('N')
[IO.File]::WriteAllText($rotateLog, $marker + "`r`n" + ('x' * 1100000) + "`r`n")
$env:NAMEFIX_LOG = $rotateLog
$paths = foreach ($i in 1..8) { New-TestFile $d (ConvertTo-Nfd "교체 $i.txt") }
$procs = foreach ($p in $paths) { Start-Process -FilePath $cli -ArgumentList "--quiet `"$p`"" -WindowStyle Hidden -PassThru }
$procs | Wait-Process -Timeout 60
$env:NAMEFIX_LOG = $logPath
$rotated = Read-TextOrNull ($rotateLog + '.1')
$current = @(if (Test-Path -LiteralPath $rotateLog) { Get-Content -LiteralPath $rotateLog -Encoding utf8 | Where-Object { $_ -match "`tSUMMARY`t" } })
Test-Case '로그 교체 시 이전 기록을 .1로 보존한다' ($null -ne $rotated -and $rotated.Contains($marker))
Test-Case '동시 실행 8건의 실행 요약이 모두 새 로그에 남는다' ($current.Count -eq 8) "summaries=$($current.Count)"

# ---------------------------------------------------------------------------
# 23. 미리 보기와 실제 실행의 충돌 판정 일치: 같은 새 이름으로 바뀔 예정인 항목끼리
# ---------------------------------------------------------------------------
$d = New-CaseDir 'planned-collision'
[void](New-TestFile $d 'A%20x.txt' 'first')
[void](New-TestFile $d '%41 x.txt' 'second')
$rDry = Invoke-Cli -n -c $d
$rRun = Invoke-Cli -c $d
$remaining = @(Get-Names $d)
Test-Case '같은 새 이름으로 바뀔 두 항목을 미리 보기에서 충돌로 보고한다' ($rDry.Code -eq 1) $rDry.Output
Test-Case '같은 새 이름으로 바뀔 두 항목은 실제 실행에서도 한 건만 바꾸고 충돌로 보고한다' ($rRun.Code -eq 1 -and (Test-HasName $d 'A x.txt') -and $remaining.Count -eq 2) (($remaining -join ', ') + ' | ' + $rRun.Output)

# 원본 이름은 서로 다르고(B%20y.txt, %62 y.txt), 정리한 이름만 대소문자가 다릅니다(B y.txt, b y.txt).
$d = New-CaseDir 'planned-collision-case'
[void](New-TestFile $d 'B%20y.txt' 'upper')
[void](New-TestFile $d '%62 y.txt' 'lower')
Test-Case '전제: 대소문자만 다른 새 이름 시험용 원본 두 개가 모두 있다' (@(Get-Names $d).Count -eq 2)
$rDry = Invoke-Cli -n -c $d
$rRun = Invoke-Cli -c $d
Test-Case '대소문자를 구분하지 않는 폴더에서 대소문자만 다른 새 이름은 미리 보기와 실제 실행 모두 충돌이다' ($rDry.Code -eq 1 -and $rRun.Code -eq 1) ($rDry.Output + ' | ' + $rRun.Output)

# 같은 폴더를 대소문자가 다른 경로 표기(…\casefold, …\CASEFOLD)로 지정해도 같은 폴더로 보고 예약을 공유한다
$d = New-CaseDir 'casefold'
$dUpper = Join-Path $work 'CASEFOLD'
[void](New-TestFile $d 'A%20z.txt' 'first')
[void](New-TestFile $d '%41 z.txt' 'second')
$rDry = Invoke-Cli -n (Join-Path $d 'A%20z.txt') (Join-Path $dUpper '%41 z.txt')
Test-Case '부모 경로 표기가 달라도 같은 폴더의 예정 이름 충돌을 미리 보기에서 보고한다' ($rDry.Code -eq 1) $rDry.Output

# 같은 폴더를 정션 경로로 지정해도 실제 폴더와 예약을 공유한다
$d = New-CaseDir 'junction-planned'
$real = New-TestDir $d 'real'
[void](New-TestFile $real 'A%20j.txt' 'first')
[void](New-TestFile $real '%41 j.txt' 'second')
$alias = Join-Path $d 'alias'
[void](New-Item -ItemType Junction -Path $alias -Target $real)
$rDry = Invoke-Cli -n (Join-Path $real 'A%20j.txt') (Join-Path $alias '%41 j.txt')
Test-Case '정션 경로로 지정한 같은 폴더의 예정 이름 충돌도 미리 보기에서 보고한다' ($rDry.Code -eq 1) $rDry.Output

# ---------------------------------------------------------------------------
# 24. 제거(uninstall.bat): 격리된 레지스트리 경로와 가짜 LOCALAPPDATA로 실패·성공 경로를 확인
#     NAMEFIX_TEST_REGISTRY_ROOT는 HKCU\Software\namefix-test* 아래만 허용됩니다. 실제 메뉴 등록은 건드리지 않습니다.
# ---------------------------------------------------------------------------
# 안전장치: 격리 경로 환경 변수를 모르는 실행 파일(이전 빌드)로 이 구간을 실행하면 실제 메뉴 등록을 바꾸게 됩니다.
# 실행 파일에 환경 변수 이름이 들어 있는지(UTF-16 문자열 리터럴) 먼저 확인하고, 없으면 이 구간을 건너뜁니다.
# 문자열 리터럴은 홀수 바이트 위치에서 시작할 수 있으므로 두 정렬을 모두 확인합니다.
$cliBytes = [IO.File]::ReadAllBytes($cli)
$supportsSandbox = [Text.Encoding]::Unicode.GetString($cliBytes).Contains('NAMEFIX_TEST_REGISTRY_ROOT') -or
    [Text.Encoding]::Unicode.GetString($cliBytes, 1, $cliBytes.Length - 1).Contains('NAMEFIX_TEST_REGISTRY_ROOT')
if (-not $supportsSandbox) {
    Test-Case '제거 테스트: 실행 파일이 격리 레지스트리 경로를 지원한다(미지원 시 이 구간을 건너뜀)' $false '실행 파일을 다시 빌드하십시오.'
}
else {
$sandboxRoot = 'Software\namefix-test-' + [guid]::NewGuid().ToString('N').Substring(0, 8)
$fakeLocal = New-CaseDir 'fake-localappdata'
$fakeDest = Join-Path $fakeLocal 'Programs\namefix'
[void][IO.Directory]::CreateDirectory($fakeDest)
Copy-Item -LiteralPath $cli, $gui -Destination $fakeDest
$savedLocal = $env:LOCALAPPDATA
$env:NAMEFIX_TEST_REGISTRY_ROOT = $sandboxRoot
$uninstall = Join-Path $root 'uninstall.bat'
$denyRule = [Security.AccessControl.RegistryAccessRule]::new(
    [Security.Principal.WindowsIdentity]::GetCurrent().User,
    [Security.AccessControl.RegistryRights]::Delete,
    [Security.AccessControl.AccessControlType]::Deny)
# 격리 경로의 빈 공간 메뉴 키에 "삭제 거부" 규칙을 넣거나 뺍니다.
# pwsh의 Set-Acl -LiteralPath는 레지스트리 키를 찾지 못하는 경우가 있어 .NET API를 직접 씁니다.
function Set-DeleteDeny([bool]$Deny) {
    $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("$sandboxRoot\Directory\Background\shell\namefix",
        [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
        [Security.AccessControl.RegistryRights]'ReadPermissions, ChangePermissions')
    if ($null -eq $k) { return }
    try {
        $acl = $k.GetAccessControl()
        if ($Deny) { $acl.AddAccessRule($denyRule) } else { [void]$acl.RemoveAccessRule($denyRule) }
        $k.SetAccessControl($acl)
    }
    finally { $k.Dispose() }
}
function Test-SandboxKey([string]$Sub) {
    $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("$sandboxRoot\$Sub")
    if ($null -eq $k) { return $false }
    $k.Dispose()
    $true
}
try {
    $reg = Invoke-Cli --register
    # 등록은 설치 폴더가 아닌 bin\ 기준이지만, 여기서는 키 생성 여부만 확인합니다.
    Test-Case '격리 경로에 메뉴 키 4개를 등록한다' ($reg.Code -eq 0 -and (Test-SandboxKey '*\shell\namefix\command') -and (Test-SandboxKey 'Directory\Background\shell\namefix\command')) $reg.Output

    # 24-0. 격리 경로 값이 허용 범위가 아니면 실제 메뉴(Software\Classes)로 되돌아가지 않고 실패한다
    function Get-RealFileVerbCommand {
        $k = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Software\Classes\*\shell\namefix\command')
        if ($null -eq $k) { return '<none>' }
        try { [string]$k.GetValue('') } finally { $k.Dispose() }
    }
    $realBefore = Get-RealFileVerbCommand
    $env:NAMEFIX_TEST_REGISTRY_ROOT = 'Software\Classes-typo'
    $bad = Invoke-Cli --register
    $env:NAMEFIX_TEST_REGISTRY_ROOT = $sandboxRoot
    $realAfter = Get-RealFileVerbCommand
    Test-Case '격리 경로 값이 잘못되면 실제 메뉴를 바꾸지 않고 종료 코드 1을 반환한다' ($bad.Code -eq 1 -and [string]::Equals($realBefore, $realAfter, [StringComparison]::Ordinal)) ($bad.Output + " | before=$realBefore after=$realAfter")

    # 24-1. 레지스트리 키 삭제 권한이 거부되면 실패로 끝내고 설치 파일을 남긴다
    Set-DeleteDeny $true
    $env:LOCALAPPDATA = $fakeLocal
    $out = & cmd.exe /c "`"$uninstall`"" 2>&1 | Out-String
    $code = $LASTEXITCODE
    $env:LOCALAPPDATA = $savedLocal
    Test-Case '제거: 메뉴 키 삭제가 거부되면 종료 코드 1을 반환하고 설치 파일을 남긴다' ($code -eq 1 -and (Test-Path -LiteralPath (Join-Path $fakeDest 'namefixw.exe'))) $out

    # 24-2. 권한을 되돌리고, 실행 파일을 잠가 폴더 삭제가 실패하게 만든다
    Set-DeleteDeny $false
    $lock =[IO.File]::Open((Join-Path $fakeDest 'namefixw.exe'), [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
    try {
        $env:LOCALAPPDATA = $fakeLocal
        $out = & cmd.exe /c "`"$uninstall`"" 2>&1 | Out-String
        $code = $LASTEXITCODE
        $env:LOCALAPPDATA = $savedLocal
    }
    finally { $lock.Dispose() }
    Test-Case '제거: 설치 폴더를 지우지 못하면 종료 코드 1을 반환한다' ($code -eq 1 -and (Test-Path -LiteralPath $fakeDest) -and -not (Test-SandboxKey '*\shell\namefix')) $out

    # 24-3. 잠금을 풀고 다시 실행하면 성공한다(설치 폴더의 namefix.exe가 이미 지워진 경우 bin\ 실행 파일로 해제)
    $env:LOCALAPPDATA = $fakeLocal
    $out = & cmd.exe /c "`"$uninstall`"" 2>&1 | Out-String
    $code = $LASTEXITCODE
    $env:LOCALAPPDATA = $savedLocal
    Test-Case '제거: 다시 실행하면 종료 코드 0으로 메뉴 키와 설치 폴더를 모두 지운다' ($code -eq 0 -and -not (Test-Path -LiteralPath $fakeDest) -and -not (Test-SandboxKey 'Directory\Background\shell\namefix')) $out
}
finally {
    $env:LOCALAPPDATA = $savedLocal
    Remove-Item Env:\NAMEFIX_TEST_REGISTRY_ROOT -ErrorAction SilentlyContinue
    try { Set-DeleteDeny $false } catch { }
    [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree($sandboxRoot, $false)
}
}

# ---------------------------------------------------------------------------
# 결과
# ---------------------------------------------------------------------------
Write-Host ''
Write-Host ("결과: 통과 {0}건, 실패 {1}건" -f $script:passed, $script:failed.Count)
Remove-Item Env:\NAMEFIX_LOG, Env:\NAMEFIX_NO_DIALOG -ErrorAction SilentlyContinue

if ($script:failed.Count -eq 0 -and -not $Keep) {
    # SMB 공유 시험 직후에는 공유 클라이언트가 핸들을 잠시 쥐고 있을 수 있으므로 몇 번 다시 시도합니다.
    $removed = $false
    for ($attempt = 0; $attempt -lt 10 -and -not $removed; $attempt++) {
        try {
            Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction Stop
            $removed = $true
        }
        catch { Start-Sleep -Milliseconds 500 }
    }
    if (-not $removed) { Write-Host "테스트 폴더를 지우지 못했습니다(다른 프로세스가 사용 중): $work" }
}
else {
    Write-Host "테스트 폴더를 남겼습니다: $work"
}
exit ([int]($script:failed.Count -gt 0))
