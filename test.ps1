# Marchat Test Runner (PowerShell)
# This script runs all tests in the Marchat project

param(
    [switch]$Coverage,
    [switch]$Verbose
)

Write-Host "Running Marchat Test Suite" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan

# Check if Go is installed
try {
    $goVersion = go version
    Write-Host "Using Go: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Run tests
Write-Host ""
Write-Host "Running tests..." -ForegroundColor Yellow
Write-Host "=================" -ForegroundColor Yellow

$testArgs = @()
if ($Verbose) {
    $testArgs += "-v"
}

try {
    if ($Coverage) {
        $testArgs += "-coverprofile=coverage.out"
    }
    
    $testArgs += "./..."
    
    & go test @testArgs
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host ""
        Write-Host "All tests passed!" -ForegroundColor Green
        
        if ($Coverage) {
            Write-Host ""
            Write-Host "Generating coverage report..." -ForegroundColor Yellow
            Write-Host "=============================" -ForegroundColor Yellow
            
            go tool cover -html=coverage.out -o coverage.html
            
            Write-Host "Coverage report generated: coverage.html" -ForegroundColor Green
            
            # Show coverage summary
            Write-Host ""
            Write-Host "Coverage Summary:" -ForegroundColor Yellow
            Write-Host "=================" -ForegroundColor Yellow
            $coverageSummary = go tool cover -func=coverage.out | Select-Object -Last 1
            Write-Host $coverageSummary -ForegroundColor White
        }
        
        Write-Host ""
        Write-Host "Test suite completed successfully!" -ForegroundColor Green
    } else {
        Write-Host ""
        Write-Host "Some tests failed!" -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host ""
    Write-Host "Error running tests: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
