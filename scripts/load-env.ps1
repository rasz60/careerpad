# .env 파일을 현재 PowerShell 세션의 환경변수로 올린다 (Windows 개발용. 서버는 pm2 가 읽는다).
# 백엔드는 os.Getenv 로만 설정을 읽으므로, go run 전에 한 번 실행해야 한다.
#
#   사용법:  . .\scripts\load-env.ps1             (저장소 루트에서)
#            . ..\scripts\load-env.ps1            (backend 폴더에서 — 어디서 불러도 저장소 루트의 .env 를 읽는다)
#            . .\scripts\load-env.ps1 .env.prod   (다른 파일)
#
# 앞의 점(.)은 dot-sourcing 이다. 빼면 자식 프로세스에만 적용되어 효과가 없다.

param(
    [string]$Path = (Join-Path $PSScriptRoot "..\.env")
)

if (-not (Test-Path $Path)) {
    Write-Error "env 파일을 찾을 수 없습니다: $Path  (.env.example 을 복사하세요)"
    return
}

$loaded = 0
# PowerShell 5.1 은 인코딩을 안 주면 ANSI 로 읽어 한글 값(MAIL_FROM 등)이 깨진다
foreach ($line in Get-Content -Path $Path -Encoding UTF8) {
    $trimmed = $line.Trim()
    if ($trimmed -eq "" -or $trimmed.StartsWith("#")) { continue }

    $pair = $trimmed -split "=", 2
    if ($pair.Count -ne 2) { continue }

    $key = $pair[0].Trim()
    # 값에 따옴표가 있으면 벗긴다.
    $value = $pair[1].Trim().Trim('"').Trim("'")

    Set-Item -Path "env:$key" -Value $value
    $loaded++
}

Write-Host "환경변수 $loaded 개를 불러왔습니다 ($Path)" -ForegroundColor Green
