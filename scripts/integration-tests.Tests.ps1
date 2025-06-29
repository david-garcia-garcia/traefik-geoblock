BeforeAll {
    # Test configuration
    $script:BaseUrl = "http://localhost:8000"
    $script:TraefikApiUrl = "http://localhost:8080"
    
    # Test IPs
    $script:TestIPs = @{
        US_Google_DNS = "8.8.8.8"
        German_IP = "85.214.132.117"
        Private_IP = "192.168.1.100"
        Localhost = "127.0.0.1"
    }
    
    # Helper function to make HTTP requests with proper error handling
    function Invoke-TestRequest {
        param(
            [string]$Uri,
            [hashtable]$Headers = @{},
            [int]$TimeoutSec = 10
        )
        
        try {
            $response = Invoke-WebRequest -Uri $Uri -Headers $Headers -Method Get -TimeoutSec $TimeoutSec -UseBasicParsing
            return @{
                StatusCode = $response.StatusCode
                Content = $response.Content
                Success = $true
                Error = $null
            }
        }
        catch {
            $statusCode = 0
            $content = ""
            
            if ($_.Exception.Response) {
                $statusCode = [int]$_.Exception.Response.StatusCode
                try {
                    # For PowerShell 7+ with HttpResponseMessage
                    if ($_.Exception.Response.Content) {
                        $content = $_.Exception.Response.Content.ReadAsStringAsync().Result
                    }
                    else {
                        # Fallback for older PowerShell versions
                        $stream = $_.Exception.Response.GetResponseStream()
                        $reader = New-Object System.IO.StreamReader($stream)
                        $content = $reader.ReadToEnd()
                        $reader.Close()
                        $stream.Close()
                    }
                }
                catch {
                    $content = $_.Exception.Message
                }
            }
            
            return @{
                StatusCode = $statusCode
                Content = $content
                Success = $false
                Error = $_.Exception.Message
            }
        }
    }
}

Describe "Traefik Geoblock Plugin Integration Tests" {
    
    Context "Basic Connectivity" {
        It "Should allow access to /foo endpoint from localhost (private IP allowed)" {
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo"
            $result.StatusCode | Should -Be 200
        }
        
        It "Should allow access to /bar endpoint from localhost (private IP allowed)" {
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/bar"
            $result.StatusCode | Should -Be 200
        }
        
        It "Should have Traefik API accessible" {
            $result = Invoke-TestRequest -Uri "$script:TraefikApiUrl/api/rawdata"
            $result.StatusCode | Should -Be 200
        }
    }
    
    Context "Geoblocking with X-Real-IP Header" {
        It "Should block US IP (Google DNS) on /foo" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should block US IP (Google DNS) on /bar" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/bar" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should allow German IP on /foo" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
        
        It "Should allow private IP range" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.Private_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
    }
    
    Context "Geoblocking with X-Forwarded-For Header" {
        It "Should block US IP via X-Forwarded-For header" {
            $headers = @{ "X-Forwarded-For" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should handle multiple IPs in X-Forwarded-For (first IP blocked)" {
            $headers = @{ "X-Forwarded-For" = "$($script:TestIPs.US_Google_DNS), $($script:TestIPs.German_IP)" }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
    }
    
    Context "Ban HTML Response" {
        It "Should serve custom ban HTML for blocked requests" {
            # Use curl directly to get the response content
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo") -join "`n"
            $statusCode = curl -s -o nul -w "%{http_code}" -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo"
            
            $statusCode | Should -Be "403"
            $response | Should -Match "Access Denied"
            $response | Should -Match $script:TestIPs.US_Google_DNS
        }
        
        It "Should include country information in ban response" {
            # Use curl directly to get the response content
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo") -join "`n"
            $statusCode = curl -s -o nul -w "%{http_code}" -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo"
            
            $statusCode | Should -Be "403"
            # The response should contain country info (US for Google DNS)
            $response | Should -Match "(US|United States)"
        }
    }
    
    Context "Auto-update Configuration" {
        It "Should work with auto-update enabled endpoint (/bar)" {
            # Test that the auto-update endpoint still blocks appropriately
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/bar" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should allow legitimate traffic on auto-update endpoint" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/bar" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
    }
    
    Context "Edge Cases and Error Handling" {
        It "Should handle malformed IP addresses gracefully" {
            $headers = @{ "X-Real-IP" = "not.an.ip.address" }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo" -Headers $headers
            # Should either block (403) or allow (200) depending on banIfError setting
            $result.StatusCode | Should -BeIn @(200, 403)
        }
        
        It "Should handle missing IP headers (localhost access allowed)" {
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo"
            $result.StatusCode | Should -Be 200
        }
        
        It "Should handle empty X-Real-IP header (localhost allowed)" {
            $headers = @{ "X-Real-IP" = "" }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
    }
    
    Context "Geoblock Log File Testing" {
        It "Should write blocked requests to custom log file" {
            # Clear any existing log content by triggering a container restart if needed
            # Make a blocked request to the logtest endpoint
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/logtest" -Headers $headers
            $result.StatusCode | Should -Be 403
            
            # Wait a moment for log to be written
            Start-Sleep -Seconds 2
            
            # Read the log file from the Docker container
            $logContent = docker exec traefik cat /var/log/geoblock/geoblock.log 2>$null
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to read geoblock log file from container"
            }
            
            # Parse the JSON log entries
            $logLines = $logContent -split "`n" | Where-Object { $_.Trim() -ne "" }
            $logLines.Count | Should -BeGreaterThan 0
            
            # Validate that ALL log lines are properly formatted JSON (no malformed lines should exist)
            $allLogEntries = @()
            foreach ($line in $logLines) {
                try {
                    $logEntry = $line | ConvertFrom-Json
                    $allLogEntries += $logEntry
                } catch {
                    throw "Malformed JSON line found in log file: '$line'. This indicates a bug in the logging implementation."
                }
            }
            
            # Find the blocked request log entry
            $blockedEntry = $allLogEntries | Where-Object { 
                $_.msg -eq "blocked request" -and $_.ip -eq $script:TestIPs.US_Google_DNS 
            } | Select-Object -First 1
            
            # Verify the log entry contains expected fields
            $blockedEntry | Should -Not -BeNullOrEmpty
            $blockedEntry.level | Should -Be "INFO"
            $blockedEntry.msg | Should -Be "blocked request"
            $blockedEntry.plugin | Should -Match "geoblock"
            $blockedEntry.ip | Should -Be $script:TestIPs.US_Google_DNS
            $blockedEntry.country | Should -Be "US"
            $blockedEntry.host | Should -Match "localhost"
            $blockedEntry.method | Should -Be "GET"
            $blockedEntry.path | Should -Be "/logtest"
            $blockedEntry.phase | Should -Be "blocked_country"
        }
        
        It "Should not log allowed requests (only blocked requests are logged)" {
            # Make an allowed request to the logtest endpoint
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/logtest" -Headers $headers
            $result.StatusCode | Should -Be 200
            
            # Wait a moment for any potential log to be written
            Start-Sleep -Seconds 2
            
            # Read the log file from the Docker container
            $logContent = docker exec traefik cat /var/log/geoblock/geoblock.log 2>$null
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to read geoblock log file from container"
            }
            
            # Parse the log lines and check for any entries related to the German IP
            $logLines = $logContent -split "`n" | Where-Object { $_.Trim() -ne "" }
            
            # Validate that ALL log lines are properly formatted JSON (no malformed lines should exist)
            $allLogEntries = @()
            foreach ($line in $logLines) {
                try {
                    $logEntry = $line | ConvertFrom-Json
                    $allLogEntries += $logEntry
                } catch {
                    throw "Malformed JSON line found in log file: '$line'. This indicates a bug in the logging implementation."
                }
            }
            
            # Look for any log entries with the German IP (should not find any for allowed requests)
            $germanIPLogFound = ($allLogEntries | Where-Object { $_.ip -eq $script:TestIPs.German_IP }).Count -gt 0
            
            # Verify that allowed requests are NOT logged
            $germanIPLogFound | Should -Be $false
        }
        
        It "Should include correct timestamp format in log entries" {
            # Make a blocked request to generate a log entry
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/logtest" -Headers $headers
            $result.StatusCode | Should -Be 403
            
            # Wait a moment for log to be written
            Start-Sleep -Seconds 2
            
            # Read and parse the log file
            $logContent = docker exec traefik cat /var/log/geoblock/geoblock.log 2>$null
            $logLines = $logContent -split "`n" | Where-Object { $_.Trim() -ne "" }
            
            # Validate that ALL log lines are properly formatted JSON (no malformed lines should exist)
            $allLogEntries = @()
            foreach ($line in $logLines) {
                try {
                    $logEntry = $line | ConvertFrom-Json
                    $allLogEntries += $logEntry
                } catch {
                    throw "Malformed JSON line found in log file: '$line'. This indicates a bug in the logging implementation."
                }
            }
            
            # Find a geoblock log entry
            $geoblockEntry = $allLogEntries | Where-Object { 
                $_.plugin -match "geoblock" -and $_.time 
            } | Select-Object -First 1
            
            # Verify timestamp format (ISO 8601)
            $geoblockEntry | Should -Not -BeNullOrEmpty
            $geoblockEntry.time | Should -Match '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}'
            
            # Verify the timestamp can be parsed as a valid DateTime
            $timestamp = [DateTime]::Parse($geoblockEntry.time)
            $timestamp | Should -BeOfType [DateTime]
            
            # Verify the timestamp is recent (within last 5 minutes)
            $timeDiff = [DateTime]::UtcNow - $timestamp.ToUniversalTime()
            $timeDiff.TotalMinutes | Should -BeLessThan 5
        }
        
        It "Should add countryHeader to allowed requests" {
            # Make an allowed request to the countryHeaderTest endpoint
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/countryHeaderTest" -Headers $headers
            $result.StatusCode | Should -Be 200
            
            # Wait a moment for any potential log to be written
            Start-Sleep -Seconds 2
            
            # Read the access.log file from the traefik container
            $accessLogContent = docker exec traefik cat /var/log/traefik/access.log 2>$null
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to read traefik access log"
            }
            
            # Parse the log lines and check for any entries related to the German IP
            $logLines = $accessLogContent -split "`n" | Where-Object { $_.Trim() -ne "" }
            
            # Validate that ALL log lines are properly formatted JSON (no malformed lines should exist)
            $allLogEntries = @()
            foreach ($line in $logLines) {
                try {
                    $logEntry = $line | ConvertFrom-Json
                    $allLogEntries += $logEntry
                } catch {
                    throw "Malformed JSON line found in log file: '$line'."
                }
            }
            
            # Look for log entries where the X-IPCountry header for Germany is added to the request
            $countryHeaderLogFound = ($allLogEntries | Where-Object { $_.'request_X-Ipcountry' -eq "DE" }).Count -gt 0
            
            # Verify that the country header was added to the request
            $countryHeaderLogFound | Should -Be $true
        }

        It "Should add countryHeader to blocked requests" {
            # Make an allowed request to the countryHeaderTest endpoint
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/countryHeaderTest" -Headers $headers
            $result.StatusCode | Should -Be 403
            
            # Wait a moment for any potential log to be written
            Start-Sleep -Seconds 2
            
            # Read the access.log file from the traefik container
            $accessLogContent = docker exec traefik cat /var/log/traefik/access.log 2>$null
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to read traefik access log"
            }
            
            # Parse the log lines and check for any entries related to the German IP
            $logLines = $accessLogContent -split "`n" | Where-Object { $_.Trim() -ne "" }
            
            # Validate that ALL log lines are properly formatted JSON (no malformed lines should exist)
            $allLogEntries = @()
            foreach ($line in $logLines) {
                try {
                    $logEntry = $line | ConvertFrom-Json
                    $allLogEntries += $logEntry
                } catch {
                    throw "Malformed JSON line found in log file: '$line'."
                }
            }
            
            # Look for log entries where the X-IPCountry header for US is added to the request
            $countryHeaderLogFound = ($allLogEntries | Where-Object { $_.'request_X-Ipcountry' -eq "US" }).Count -gt 0
            
            # Verify that the country header was added to the request
            $countryHeaderLogFound | Should -Be $true
        }

        It "Should add countryHeader with PRIVATE value to local requests" {
            # Make an allowed request to the countryHeaderTest endpoint
            $headers = @{ "X-Real-IP" = $script:TestIPs.Private_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/countryHeaderTest" -Headers $headers
            $result.StatusCode | Should -Be 200
            
            # Wait a moment for any potential log to be written
            Start-Sleep -Seconds 2
            
            # Read the access.log file from the traefik container
            $accessLogContent = docker exec traefik cat /var/log/traefik/access.log 2>$null
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to read traefik access log"
            }
            
            # Parse the log lines and check for any entries related to the private IP
            $logLines = $accessLogContent -split "`n" | Where-Object { $_.Trim() -ne "" }
            
            # Validate that ALL log lines are properly formatted JSON (no malformed lines should exist)
            $allLogEntries = @()
            foreach ($line in $logLines) {
                try {
                    $logEntry = $line | ConvertFrom-Json
                    $allLogEntries += $logEntry
                } catch {
                    throw "Malformed JSON line found in log file: '$line'."
                }
            }
            
            # Look for log entries where the X-IPCountry header for PRIVATE is added to the request
            $countryHeaderLogFound = ($allLogEntries | Where-Object { $_.'request_X-Ipcountry' -eq "PRIVATE" }).Count -gt 0
            
            # Verify that the country header was added with PRIVATE value
            $countryHeaderLogFound | Should -Be $true
        }
    }
    
    Context "Block All Requests" {
        It "Should block localhost request (private IP not allowed)" {
            # The /blockall endpoint has allowPrivate=false, so even localhost should be blocked
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/blockall"
            $result.StatusCode | Should -Be 403
        }
        
        It "Should block German IP (normally allowed elsewhere)" {
            # German IP is normally allowed in other endpoints, but should be blocked here
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/blockall" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should block US IP" {
            # US IP should be blocked (consistent with other endpoints)
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/blockall" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should block private IP range" {
            # Private IP should be blocked since allowPrivate=false
            $headers = @{ "X-Real-IP" = $script:TestIPs.Private_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/blockall" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should serve ban HTML for blocked requests with country info" {
            # Use curl to get the response content for a German IP
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.German_IP)" "$script:BaseUrl/blockall") -join "`n"
            $statusCode = curl -s -o nul -w "%{http_code}" -H "X-Real-IP: $($script:TestIPs.German_IP)" "$script:BaseUrl/blockall"
            
            $statusCode | Should -Be "403"
            $response | Should -Match "Access Denied"
            $response | Should -Match $script:TestIPs.German_IP
            $response | Should -Match "DE"  # Should contain German country code
        }
        
        It "Should serve ban HTML for blocked private IP requests" {
            # Use curl to get the response content for a private IP
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.Private_IP)" "$script:BaseUrl/blockall") -join "`n"
            $statusCode = curl -s -o nul -w "%{http_code}" -H "X-Real-IP: $($script:TestIPs.Private_IP)" "$script:BaseUrl/blockall"
            
            $statusCode | Should -Be "403"
            $response | Should -Match "Access Denied"
            $response | Should -Match $script:TestIPs.Private_IP
            $response | Should -Match "PRIVATE"  # Should contain PRIVATE for private IP
        }
    }
    
    Context "Performance and Reliability" {
        It "Should respond within reasonable time" {
            $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/foo"
            $stopwatch.Stop()
            
            $result.StatusCode | Should -Be 200  # Allowed due to private IP
            $stopwatch.ElapsedMilliseconds | Should -BeLessThan 5000  # 5 seconds max
        }
        
        It "Should handle concurrent requests" {
            $jobs = @()
            1..5 | ForEach-Object {
                $jobs += Start-Job -ScriptBlock {
                    param($BaseUrl)
                    try {
                        $response = Invoke-WebRequest -Uri "$BaseUrl/foo" -Method Get -TimeoutSec 10 -UseBasicParsing
                        return $response.StatusCode
                    } catch {
                        if ($_.Exception.Response) {
                            return [int]$_.Exception.Response.StatusCode
                        }
                        return 500
                    }
                } -ArgumentList $script:BaseUrl
            }
            
            $results = $jobs | Wait-Job | Receive-Job
            $jobs | Remove-Job
            
            # All requests should succeed (200) since they're from localhost (private IP allowed)
            $results | ForEach-Object { $_ | Should -Be 200 }
        }
    }
} 