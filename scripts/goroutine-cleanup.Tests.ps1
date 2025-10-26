#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Integration test to verify goroutine cleanup on Traefik config reload
    
.DESCRIPTION
    Verifies that bufferedFileWriter goroutines are properly cleaned up when
    Traefik reloads configuration via docker compose hot reloads.
#>

Describe "Goroutine Cleanup" {
    BeforeAll {
        $script:TestServiceName = "whoami-logtest"
        $script:TestServicePath = "/logtest"
        $script:LogFile = "geoblock.log"
    }
    
    Context "Context Cancellation on Hot Reload" {
        # Tests simulate production hot reloads: add label, stop, start
        # Traefik detects container change and cancels context for old middleware
        It "Should start goroutine when plugin is loaded" {
            $testUrl = "http://localhost:8000$script:TestServicePath"
            
            # Verify service is accessible
            $response = Invoke-WebRequest -Uri $testUrl -Method Get -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
            $response.StatusCode | Should -Be 200
            
            Start-Sleep -Seconds 2
            
            # Check logs for goroutine start message
            $logs = docker logs traefik 2>&1 | Select-String "bufferedFileWriter.*starting flush timer goroutine"
            
            $logs | Should -Not -BeNullOrEmpty
            $logs | Should -Match "starting flush timer goroutine"
        }
        
        It "Should exit goroutine when Traefik config is reloaded" {
            # Simulate config change: add label, stop, start
            # This is what happens when you edit docker-compose.yml and run 'docker compose up -d'
            docker update --label-add "test.reload.marker=reload1" $script:TestServiceName 2>&1 | Out-Null
            docker compose stop $script:TestServiceName 2>&1 | Out-Null
            Start-Sleep -Seconds 2
            docker compose up -d $script:TestServiceName 2>&1 | Out-Null
            Start-Sleep -Seconds 4
            
            # Check logs for goroutine exit message
            $logs = docker logs traefik 2>&1
            $exitLogs = $logs | Select-String "bufferedFileWriter.*goroutine exiting due to context cancellation"
            
            $exitLogs | Should -Not -BeNullOrEmpty
            $exitLogs | Should -Match "goroutine exiting due to context cancellation"
            $exitLogs | Should -Match "context canceled"
        }
        
        It "Should handle multiple config reloads without leaking goroutines" {
            # Perform 3 hot reloads with label change
            for ($i = 1; $i -le 3; $i++) {
                docker update --label-add "test.reload.marker=reload$i" $script:TestServiceName 2>&1 | Out-Null
                docker compose stop $script:TestServiceName 2>&1 | Out-Null
                Start-Sleep -Seconds 2
                docker compose up -d $script:TestServiceName 2>&1 | Out-Null
                Start-Sleep -Seconds 4
            }
            
            # Count goroutine starts and exits
            $logs = docker logs traefik 2>&1
            $starts = ($logs | Select-String "bufferedFileWriter.*starting flush timer goroutine.*$script:LogFile").Count
            $exits = ($logs | Select-String "bufferedFileWriter.*goroutine exiting.*$script:LogFile").Count
            
            # Should have multiple starts and exits
            $starts | Should -BeGreaterThan 2
            $exits | Should -BeGreaterThan 0
            
            # Difference should be small (only current goroutine running)
            ($starts - $exits) | Should -BeLessOrEqual 2
        }
        
        It "Should flush buffer and log final size on goroutine exit" {
            # Make requests to generate log data
            $testUrl = "http://localhost:8000$script:TestServicePath"
            1..5 | ForEach-Object {
                Invoke-WebRequest -Uri $testUrl -Method Get -TimeoutSec 5 -UseBasicParsing -ErrorAction SilentlyContinue | Out-Null
            }
            
            Start-Sleep -Seconds 1
            
            # Trigger hot reload
            docker update --label-add "test.reload.marker=final" $script:TestServiceName 2>&1 | Out-Null
            docker compose stop $script:TestServiceName 2>&1 | Out-Null
            Start-Sleep -Seconds 2
            docker compose up -d $script:TestServiceName 2>&1 | Out-Null
            Start-Sleep -Seconds 4
            
            # Check for final_buffer_size in exit logs
            $logs = docker logs traefik 2>&1
            $exitLogs = $logs | Select-String "goroutine exiting.*final_buffer_size"
            
            $exitLogs | Should -Not -BeNullOrEmpty
            $exitLogs | Should -Match "final_buffer_size="
        }
    }
}

