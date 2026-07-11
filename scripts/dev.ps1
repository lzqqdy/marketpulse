# MarketPulse Windows 开发脚本
# 用法: .\scripts\dev.ps1 <command>
# 示例: .\scripts\dev.ps1 api | .\scripts\dev.ps1 web | .\scripts\dev.ps1 dev

param(
    [Parameter(Position = 0)]
    [string]$Command = "help"
)

$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$ConfigPath = Join-Path $Root "config\config.yaml"
$ConfigExample = Join-Path $Root "config\config.example.yaml"
$WebDir = Join-Path $Root "web"
$LogDir = Join-Path $Root "log"
$BinDir = Join-Path $Root "bin"

function Add-ToolPath([string]$Dir) {
    if (-not $Dir -or -not (Test-Path $Dir)) {
        return
    }
    if ($env:Path -notlike "*$Dir*") {
        $env:Path = "$Dir;$env:Path"
    }
}

function Resolve-ToolExe([string]$Name, [string[]]$Candidates) {
    foreach ($candidate in $Candidates) {
        if (Test-Path $candidate) {
            Add-ToolPath (Split-Path $candidate -Parent)
            return $candidate
        }
    }
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd -and $cmd.Source -match '\.(exe|cmd)$') {
        return $cmd.Source
    }
    return $null
}

$Go = if ($env:GO) {
    $env:GO
} else {
    Resolve-ToolExe "go.exe" @(
        "C:\Program Files\Go\bin\go.exe",
        "C:\Go\bin\go.exe",
        (Join-Path $env:LOCALAPPDATA "Programs\Go\bin\go.exe")
    )
}
if (-not $Go) { $Go = "go.exe" }

$Npm = Resolve-ToolExe "npm.cmd" @(
    "C:\Program Files\nodejs\npm.cmd",
    (Join-Path $env:APPDATA "nvm\nodejs\npm.cmd"),
    (Join-Path $env:LOCALAPPDATA "Programs\nodejs\npm.cmd")
)
if (-not $Npm) { $Npm = "npm.cmd" }

function Ensure-Tools([switch]$NeedGo, [switch]$NeedNode) {
    if ($NeedGo -and -not (Get-Command $Go -ErrorAction SilentlyContinue) -and -not (Test-Path $Go)) {
        throw "未找到 Go。请安装 https://go.dev/dl/ 并重启终端，或将 Go\bin 加入 PATH。"
    }
    if ($NeedNode -and -not (Get-Command $Npm -ErrorAction SilentlyContinue) -and -not (Test-Path $Npm)) {
        throw "未找到 Node.js/npm。请安装 https://nodejs.org/ 并重启终端，或将 nodejs 目录加入 PATH。"
    }
}

function Write-Info([string]$Message) {
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Err([string]$Message) {
    Write-Host "ERROR: $Message" -ForegroundColor Red
}

function Ensure-Config {
    if (-not (Test-Path $ConfigPath)) {
        if (-not (Test-Path $ConfigExample)) {
            throw "缺少配置模板: $ConfigExample"
        }
        Copy-Item $ConfigExample $ConfigPath
        Write-Info "已创建 config\config.yaml"
    }
}

function Ensure-LogDir {
    if (-not (Test-Path $LogDir)) {
        New-Item -ItemType Directory -Path $LogDir | Out-Null
    }
}

function Ensure-WebDeps {
    Ensure-Tools -NeedNode
    $vite = Join-Path $WebDir "node_modules\.bin\vite.cmd"
    if (-not (Test-Path $vite)) {
        Write-Info "安装前端依赖（首次 clone 或 node_modules 缺失）"
        Push-Location $WebDir
        try {
            if (Test-Path (Join-Path $WebDir "package-lock.json")) {
                & $Npm ci
            } else {
                & $Npm install
            }
        } finally {
            Pop-Location
        }
    }
}

function Clear-Port([int]$Port) {
    try {
        $listeners = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        foreach ($conn in $listeners) {
            $procId = $conn.OwningProcess
            if ($procId -and $procId -ne 0) {
                Write-Info "清理占用端口 ${Port}: PID $procId"
                Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
            }
        }
    } catch {
        # Get-NetTCPConnection 在部分环境不可用，忽略
    }
}

function Start-ApiProcess {
    Ensure-Tools -NeedGo
    Ensure-Config
    Push-Location $Root
    try {
        return Start-Process `
            -FilePath $Go `
            -ArgumentList @("run", "-buildvcs=false", "./cmd/marketd", "-config", "config/config.yaml") `
            -WorkingDirectory $Root `
            -PassThru `
            -NoNewWindow
    } finally {
        Pop-Location
    }
}

function Start-WebProcess {
    Ensure-WebDeps
    return Start-Process `
        -FilePath $Npm `
        -ArgumentList @("run", "dev") `
        -WorkingDirectory $WebDir `
        -PassThru `
        -NoNewWindow
}

function Stop-DevProcess([System.Diagnostics.Process]$Process, [string]$Name) {
    if (-not $Process -or $Process.HasExited) {
        return
    }
    try {
        Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
    } catch {
        Write-Err "停止 $Name 失败: $($_.Exception.Message)"
    }
}

function Show-Help {
    Write-Host @"

MarketPulse Windows 开发脚本

  开发
    .\scripts\dev.cmd api          启动后端 (:8080)
    .\scripts\dev.cmd api-log      启动后端并写入 log\local-api.log
    .\scripts\dev.cmd web          启动前端 Vite (:5173)
    .\scripts\dev.cmd dev          同时启动后端和前端（Ctrl+C 退出）

  构建
    .\scripts\dev.cmd build-web    构建前端 -> web\dist
    .\scripts\dev.cmd build-api    构建后端 -> bin\marketd.exe
    .\scripts\dev.cmd build        前端 + 后端

  其他
    .\scripts\dev.cmd setup-config 复制 config.example.yaml（若不存在）
    .\scripts\dev.cmd test         运行 Go 单元测试
    .\scripts\dev.cmd help         显示本帮助

  验证
    curl http://127.0.0.1:8080/healthz
    浏览器打开 http://localhost:5173

"@
}

function Invoke-Api {
    Ensure-Tools -NeedGo
    Ensure-Config
    Write-Info "启动后端 http://localhost:8080"
    Push-Location $Root
    try {
        & $Go run -buildvcs=false ./cmd/marketd -config config/config.yaml
    } finally {
        Pop-Location
    }
}

function Invoke-ApiLog {
    Ensure-Tools -NeedGo
    Ensure-Config
    Ensure-LogDir
    $logFile = Join-Path $LogDir "local-api.log"
    Write-Info "启动后端 http://localhost:8080，日志写入 $logFile"
    Push-Location $Root
    try {
        & $Go run -buildvcs=false ./cmd/marketd -config config/config.yaml 2>&1 |
            Tee-Object -FilePath $logFile
    } finally {
        Pop-Location
    }
}

function Invoke-Web {
    Ensure-WebDeps
    Write-Info "启动前端 http://localhost:5173"
    Push-Location $WebDir
    try {
        & $Npm run dev
    } finally {
        Pop-Location
    }
}

function Invoke-Dev {
    Clear-Port 8080
    Clear-Port 5173

    $apiProc = $null
    $webProc = $null

    try {
        Write-Info "启动后端 http://localhost:8080"
        $apiProc = Start-ApiProcess

        Write-Info "启动前端 http://localhost:5173"
        $webProc = Start-WebProcess

        Write-Info "按 Ctrl+C 停止前后端"
        while ($true) {
            if ($apiProc.HasExited) {
                throw "后端进程已退出，退出码 $($apiProc.ExitCode)"
            }
            if ($webProc.HasExited) {
                throw "前端进程已退出，退出码 $($webProc.ExitCode)"
            }
            Start-Sleep -Seconds 1
        }
    } finally {
        Write-Host ""
        Write-Info "停止本地开发进程"
        Stop-DevProcess $webProc "前端"
        Stop-DevProcess $apiProc "后端"
        Start-Sleep -Milliseconds 800
        Clear-Port 8080
        Clear-Port 5173
    }
}

function Invoke-BuildWeb {
    Ensure-WebDeps
    Write-Info "构建前端 -> web\dist"
    Push-Location $WebDir
    try {
        if (Test-Path (Join-Path $WebDir "package-lock.json")) {
            & $Npm ci
        } else {
            & $Npm install
        }
        & $Npm run build
    } finally {
        Pop-Location
    }
}

function Invoke-BuildApi {
    Ensure-Tools -NeedGo
    Ensure-Config
    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir | Out-Null
    }
    $out = Join-Path $BinDir "marketd.exe"
    Write-Info "构建后端 -> $out"
    Push-Location $Root
    try {
        & $Go build -buildvcs=false -o $out ./cmd/marketd
    } finally {
        Pop-Location
    }
}

function Invoke-Test {
    Ensure-Tools -NeedGo
    Write-Info "运行 Go 单元测试"
    Push-Location $Root
    try {
        & $Go test -buildvcs=false ./...
    } finally {
        Pop-Location
    }
}

Push-Location $Root
try {
    switch ($Command.ToLower()) {
        "help" { Show-Help }
        "setup-config" { Ensure-Config }
        "api" { Invoke-Api }
        "api-log" { Invoke-ApiLog }
        "web" { Invoke-Web }
        "dev" { Invoke-Dev }
        "build-web" { Invoke-BuildWeb }
        "build-api" { Invoke-BuildApi }
        "build" {
            Invoke-BuildWeb
            Invoke-BuildApi
        }
        "test" { Invoke-Test }
        default {
            Write-Err "未知命令: $Command"
            Show-Help
            exit 1
        }
    }
} catch {
    Write-Err $_.Exception.Message
    exit 1
} finally {
    Pop-Location
}
