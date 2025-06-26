#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Runs integration tests for the Traefik Geoblock Plugin

.DESCRIPTION
    This script starts the Docker Compose services, waits for them to be ready,
    runs the Pester integration tests, and then cleans up the services.

.PARAMETER SkipDockerCleanup
    Skip stopping Docker services after tests complete (useful for debugging)

.PARAMETER SkipWait
    Skip waiting for services to be ready (assumes they're already running)

.PARAMETER TestPath
    Path to the Pester test file (defaults to ./scripts/integration-tests.Tests.ps1)

.EXAMPLE
    ./Test-Integration.ps1
    Runs the full integration test suite

.EXAMPLE
    ./Test-Integration.ps1 -SkipDockerCleanup
    Runs tests but leaves Docker services running for debugging

.EXAMPLE
    ./Test-Integration.ps1 -SkipWait
    Runs tests assuming services are already running
#>

[CmdletBinding()]
param(
    [switch]$SkipDockerCleanup,
    [switch]$SkipWait,
    [string]$TestPath = "./scripts/integration-tests.Tests.ps1"
)

$ErrorActionPreference = "Stop"

# Colors for output
$Colors = @{
    Info = "Cyan"
    Success = "Green"
    Warning = "Yellow"
    Error = "Red"
}

function Write-Step {
    param([string]$Message, [string]$Color = "Cyan")
    Write-Host "🔄 $Message" -ForegroundColor $Color
}

function Write-Success {
    param([string]$Message)
    Write-Host "✅ $Message" -ForegroundColor $Colors.Success
}

function Write-Warning {
    param([string]$Message)
    Write-Host "⚠️  $Message" -ForegroundColor $Colors.Warning
}

function Write-Error {
    param([string]$Message)
    Write-Host "❌ $Message" -ForegroundColor $Colors.Error
}

function Test-ServiceHealth {
    param(
        [string]$Url,
        [string]$ServiceName,
        [int]$TimeoutSeconds = 60,
        [int]$RetryIntervalSeconds = 2
    )
    
    Write-Step "Waiting for $ServiceName to be ready..."
    $elapsed = 0
    
    do {
        try {
            $response = Invoke-WebRequest -Uri $Url -Method Get -TimeoutSec 5 -UseBasicParsing
            if ($response.StatusCode -eq 200) {
                Write-Success "$ServiceName is ready!"
                return $true
            }
        }
        catch {
            # Service not ready yet, continue waiting
        }
        
        Start-Sleep $RetryIntervalSeconds
        $elapsed += $RetryIntervalSeconds
        
        if ($elapsed % 10 -eq 0) {
            Write-Host "  Still waiting for $ServiceName... ($elapsed/$TimeoutSeconds seconds)" -ForegroundColor Gray
        }
        
    } while ($elapsed -lt $TimeoutSeconds)
    
    Write-Error "$ServiceName failed to become ready within $TimeoutSeconds seconds"
    return $false
}

# Main execution
try {
    Write-Host ""
    Write-Host "🚀 Traefik Geoblock Plugin Integration Test Runner" -ForegroundColor $Colors.Info
    Write-Host "=================================================" -ForegroundColor $Colors.Info
    Write-Host ""

    # Check if Pester is available
    Write-Step "Checking Pester availability..."
    try {
        Import-Module Pester -Force -ErrorAction Stop
        $pesterVersion = (Get-Module Pester).Version
        Write-Success "Pester $pesterVersion is available"
    }
    catch {
        Write-Error "Pester module not found. Installing Pester..."
        try {
            Install-Module -Name Pester -Force -Scope CurrentUser -SkipPublisherCheck
            Import-Module Pester -Force
            Write-Success "Pester installed and imported successfully"
        }
        catch {
            Write-Error "Failed to install Pester: $($_.Exception.Message)"
            exit 1
        }
    }

    # Ensure we are using Linux containers
    Write-Step "Ensuring Linux containers are enabled..."
    if (-not(Test-Path "$Env:ProgramFiles\Docker\Docker\DockerCli.exe")) {
        Get-Command docker -ErrorAction SilentlyContinue | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Docker CLI not found. Please install Docker Desktop."
            exit 1
        }
        Write-Warning "Docker CLI not found at standard location: $Env:ProgramFiles\Docker\Docker\DockerCli.exe"
        Write-Warning "Assuming Docker is already configured for Linux containers"
    }
    else {
        Write-Step "Switching to Linux Engine..."
        try {
            & "$Env:ProgramFiles\Docker\Docker\DockerCli.exe" -SwitchLinuxEngine
            Start-Sleep 2  # Give Docker a moment to switch
            Write-Success "Switched to Linux containers"
        }
        catch {
            Write-Warning "Failed to switch to Linux Engine: $($_.Exception.Message)"
            Write-Warning "Continuing with current Docker configuration..."
        }
    }

    # Check if Docker Compose is available
    Write-Step "Checking Docker Compose availability..."
    try {
        $dockerComposeVersion = docker compose version 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-Success "Docker Compose is available"
        } else {
            throw "Docker Compose not found"
        }
    }
    catch {
        Write-Error "Docker Compose is not available. Please install Docker Desktop or Docker Compose."
        exit 1
    }

    # Start Docker services
    Write-Step "Starting Docker Compose services..."
    try {
        docker compose up -d
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to start Docker services"
        }
        Write-Success "Docker services started successfully"
    }
    catch {
        Write-Error "Failed to start Docker services: $($_.Exception.Message)"
        exit 1
    }

    if (-not $SkipWait) {
        # Wait for services to be ready
        Write-Step "Waiting for services to become ready..."
        
        $servicesReady = @(
            (Test-ServiceHealth -Url "http://localhost:8080/api/rawdata" -ServiceName "Traefik API"),
            (Test-ServiceHealth -Url "http://localhost:8000/foo" -ServiceName "Whoami /foo service"),
            (Test-ServiceHealth -Url "http://localhost:8000/bar" -ServiceName "Whoami /bar service")
        )
        
        if ($servicesReady -contains $false) {
            Write-Error "One or more services failed to start properly"
            if (-not $SkipDockerCleanup) {
                Write-Step "Cleaning up Docker services..."
                docker compose down -v
            }
            exit 1
        }
        
        Write-Success "All services are ready!"
    } else {
        Write-Warning "Skipping service readiness check (assuming services are already running)"
    }

    # Run Pester tests
    Write-Step "Running Pester integration tests..."
    Write-Host ""
    
    if (-not (Test-Path $TestPath)) {
        Write-Error "Test file not found: $TestPath"
        exit 1
    }

    try {
        $pesterConfig = New-PesterConfiguration
        $pesterConfig.Run.Path = $TestPath
        $pesterConfig.Output.Verbosity = 'Detailed'
        $pesterConfig.Run.Exit = $false
        $pesterConfig.Run.PassThru = $true  # Ensure results are returned
        
        $result = Invoke-Pester -Configuration $pesterConfig
        
        Write-Host ""
        if ($result -and $result.FailedCount -eq 0) {
            Write-Success "All integration tests passed! 🎉"
            $exitCode = 0
        } elseif ($result) {
            Write-Error "$($result.FailedCount) test(s) failed out of $($result.TotalCount) total tests"
            $exitCode = 1
        } else {
            Write-Warning "Could not determine test results"
            $exitCode = 1
        }
    }
    catch {
        Write-Error "Failed to run Pester tests: $($_.Exception.Message)"
        $exitCode = 1
    }
}
catch {
    Write-Error "Unexpected error: $($_.Exception.Message)"
    $exitCode = 1
}
finally {
    # Cleanup Docker services
    if (-not $SkipDockerCleanup) {
        Write-Step "Cleaning up Docker services..."
        try {
            docker compose down -v 2>$null
            Write-Success "Docker services stopped and cleaned up"
        }
        catch {
            Write-Warning "Failed to clean up Docker services: $($_.Exception.Message)"
        }
    } else {
        Write-Warning "Skipping Docker cleanup (services left running for debugging)"
        Write-Host "To manually stop services, run: docker compose down -v" -ForegroundColor Gray
    }
    
    Write-Host ""
    Write-Host "=================================================" -ForegroundColor $Colors.Info
    if ($exitCode -eq 0) {
        Write-Host "🏁 Integration tests completed successfully!" -ForegroundColor $Colors.Success
    } else {
        Write-Host "🏁 Integration tests completed with failures!" -ForegroundColor $Colors.Error
    }
    Write-Host ""
}

exit $exitCode 