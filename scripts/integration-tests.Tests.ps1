# Pester evaluates It -Skip at discovery, before BeforeAll.
BeforeDiscovery {
    $dotEnv = Join-Path (Split-Path $PSScriptRoot -Parent) ".env"
    if (Test-Path $dotEnv) {
        Get-Content $dotEnv | ForEach-Object {
            $line = $_.Trim()
            if ($line -eq "" -or $line.StartsWith("#")) { return }
            if ($line.StartsWith("export ")) { $line = $line.Substring(7).Trim() }
            $eq = $line.IndexOf("=")
            if ($eq -lt 1) { return }
            $name = $line.Substring(0, $eq).Trim()
            $value = $line.Substring($eq + 1).Trim().Trim('"').Trim("'")
            $existing = [Environment]::GetEnvironmentVariable($name, "Process")
            if ([string]::IsNullOrWhiteSpace($existing)) {
                [Environment]::SetEnvironmentVariable($name, $value, "Process")
                Set-Item -Path "env:$name" -Value $value
            }
        }
    }
    $script:HasIP2LocationToken = -not [string]::IsNullOrWhiteSpace($env:IP2LOCATION_DOWNLOAD_TOKEN)
    $script:HasIPinfoToken = -not [string]::IsNullOrWhiteSpace($env:IPINFO_DOWNLOAD_TOKEN)
}

BeforeAll {
    function Wait-TraefikPluginLog {
        param(
            [string]$Pattern,
            [int]$TimeoutSeconds = 180,
            [int]$RetrySeconds = 3
        )
        $elapsed = 0
        do {
            $logs = docker logs traefik 2>&1 | Out-String
            if ($logs -match $Pattern) {
                return $true
            }
            Start-Sleep $RetrySeconds
            $elapsed += $RetrySeconds
        } while ($elapsed -lt $TimeoutSeconds)
        return $false
    }

    # Test configuration
    $script:BaseUrl = "http://localhost:8000"
    $script:TraefikApiUrl = "http://localhost:8080"
    
    # Test IPs
    $script:TestIPs = @{
        US_Google_DNS = "8.8.8.8"
        German_IP = "85.214.132.117"
        Private_IP = "192.168.1.100"
        Localhost = "127.0.0.1"
        Japanese_IP = "126.0.0.1"  # JP - for testing default_allow scenarios
        AWS_Blocked_IP = "3.5.140.1"  # AWS IP in blocked IP block range (3.5.140.0/24)
        MaxMindDummyGB = "81.2.69.142"   # GeoIP2-Country-Test.mmdb GB
        MaxMindDummyCN = "175.16.199.1"  # GeoIP2-Country-Test.mmdb CN
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
    
    # Helper function to read and parse Traefik access log entries
    # Returns all log entries as parsed JSON objects
    function Get-TraefikAccessLogEntries {
        param(
            [int]$WaitSeconds = 2,
            [int]$TailLines = 100
        )
        
        # Wait for log to be written
        Start-Sleep -Seconds $WaitSeconds
        
        # Read the access log from the Traefik container
        $accessLogContent = docker exec traefik tail -n $TailLines /var/log/traefik/access.log 2>$null
        if ($LASTEXITCODE -ne 0 -or -not $accessLogContent) {
            return @()
        }
        
        # Parse log lines
        $logLines = $accessLogContent -split "`n" | Where-Object { $_.Trim() -ne "" }
        
        $allLogEntries = @()
        foreach ($line in $logLines) {
            try {
                $logEntry = $line | ConvertFrom-Json
                $allLogEntries += $logEntry
            } catch {
                # Skip malformed lines silently for this helper
            }
        }
        
        return $allLogEntries
    }

    # Get-ReclaimLogEvents parses reclaim_put / reclaim_dispose lines from Traefik stdout.
    function Get-ReclaimLogEvents {
        $events = @()
        $lines = docker logs traefik 2>&1
        foreach ($line in $lines) {
            $text = "$line"
            if ($text -notmatch 'reclaim_(put|dispose|bind|orphan|reclaim)') {
                continue
            }
            $msg = $null
            $key = $null
            if ($text -match 'msg=([^\s]+)') {
                $msg = $Matches[1].Trim('"')
            }
            if ($text -match '"msg"\s*:\s*"(reclaim_[a-z]+)"') {
                $msg = $Matches[1]
            }
            if ($text -match 'key=([^\s]+)') {
                $key = $Matches[1].Trim('"')
            }
            if ($text -match '"key"\s*:\s*"([^"]+)"') {
                $key = $Matches[1]
            }
            if ($msg -and $key) {
                $events += [pscustomobject]@{ Msg = $msg; Key = $key }
            }
        }
        return $events
    }

    # Get-ReclaimPluginPuts is reclaim_put lines whose key is this middleware's plugin incarnation.
    function Get-ReclaimPluginPuts {
        return @(Get-ReclaimLogEvents | Where-Object {
            $_.Msg -eq 'reclaim_put' -and $_.Key -like 'plugin:geoblock-reclaim*'
        })
    }

    # Wait-ReclaimPluginPutCount waits until at least $Count plugin puts exist for geoblock-reclaim.
    function Wait-ReclaimPluginPutCount {
        param(
            [int]$Count,
            [int]$TimeoutSeconds = 30
        )
        $elapsed = 0
        do {
            $puts = Get-ReclaimPluginPuts
            if ($puts.Count -ge $Count) {
                return $puts
            }
            Start-Sleep 1
            $elapsed += 1
        } while ($elapsed -lt $TimeoutSeconds)
        return @(Get-ReclaimPluginPuts)
    }

    # Wait-ReclaimDispose waits until reclaim_dispose is logged for $Key.
    function Wait-ReclaimDispose {
        param(
            [string]$Key,
            [int]$TimeoutSeconds = 20
        )
        $elapsed = 0
        do {
            $hit = @(Get-ReclaimLogEvents | Where-Object { $_.Msg -eq 'reclaim_dispose' -and $_.Key -eq $Key })
            if ($hit.Count -gt 0) {
                return $true
            }
            Start-Sleep 1
            $elapsed += 1
        } while ($elapsed -lt $TimeoutSeconds)
        return $false
    }
    
    # Helper function to find access log entry matching a specific request path
    # Helper function to get the last access log entry
    # Since tests run sequentially, this returns the most recent request's log entry
    function Get-LastAccessLogEntry {
        param(
            [int]$WaitSeconds = 1
        )
        
        # Wait for log to be written
        Start-Sleep -Seconds $WaitSeconds
        
        # Read the last line of the access log from the Traefik container
        $lastLine = docker exec traefik tail -n 1 /var/log/traefik/access.log 2>$null
        
        if (-not $lastLine) {
            return $null
        }
        
        try {
            return $lastLine | ConvertFrom-Json
        } catch {
            return $null
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

        It "Should allow access to /remediationHeaderTest endpoint from localhost (private IP allowed)" {
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/remediationHeaderTest"
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

        It "Should return ban HTML body for GET requests but no body for HEAD requests" {
            # Test GET request - should return body with ban HTML content
            $getResponse = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo") -join "`n"
            $getStatusCode = curl -s -o nul -w "%{http_code}" -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo"
            
            # Test HEAD request - should return same status but no body
            $headResponse = (curl -s -I -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo") -join "`n"
            $headStatusCode = curl -s -I -o nul -w "%{http_code}" -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/foo"
            
            # Both should return 403 status
            $getStatusCode | Should -Be "403"
            $headStatusCode | Should -Be "403"
            
            # GET should return HTML content with ban information
            $getResponse | Should -Match "Access Denied"
            $getResponse | Should -Match $script:TestIPs.US_Google_DNS
            $getResponse | Should -Match "<!DOCTYPE html>"
            
            # HEAD should return headers but no HTML body content
            $headResponse | Should -Match "HTTP.*403"
            $headResponse | Should -Not -Match "Access Denied"
            $headResponse | Should -Not -Match "<!DOCTYPE html>"
            $headResponse | Should -Not -Match $script:TestIPs.US_Google_DNS
            
            # HEAD response should only contain status headers, no Content-Type or body for blocked requests
            $headResponse | Should -Not -Match "Content-Type.*text/html"
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
    
    Context "Access log headers" {
        It "Should add countryHeader to allowed requests" {
            # Make an allowed request to the countryHeaderTest endpoint
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/countryHeaderTest" -Headers $headers
            $result.StatusCode | Should -Be 200
            
            # Read access log entries using shared helper
            $allLogEntries = Get-TraefikAccessLogEntries -WaitSeconds 2
            
            # Look for log entries where the X-IPCountry header for Germany is added to the request
            $countryHeaderLogFound = ($allLogEntries | Where-Object { $_.'request_X-Ipcountry' -eq "DE" }).Count -gt 0
            
            # Verify that the country header was added to the request
            $countryHeaderLogFound | Should -Be $true
        }

        It "Should add countryHeader to blocked requests" {
            # Make a blocked request to the countryHeaderTest endpoint
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/countryHeaderTest" -Headers $headers
            $result.StatusCode | Should -Be 403
            
            # Read access log entries using shared helper
            $allLogEntries = Get-TraefikAccessLogEntries -WaitSeconds 2
            
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
            
            # Read access log entries using shared helper
            $allLogEntries = Get-TraefikAccessLogEntries -WaitSeconds 2
            
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
    
    Context "Excluded Paths Regex" {
        # The /excludedpaths endpoint has excludedPathsRegex=^/excludedpaths/(api/.*|health)$
        # This means paths matching that pattern should skip blocking but still get GeoIP enrichment
        
        It "Should block US IP on non-excluded path" {
            # /excludedpaths/blocked does NOT match the exclusion regex
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/excludedpaths/blocked" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should allow US IP on excluded /api/* path" {
            # /excludedpaths/api/users MATCHES the exclusion regex, so blocking is skipped
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/excludedpaths/api/users" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
        
        It "Should allow US IP on excluded /api/nested/path" {
            # /excludedpaths/api/nested/path MATCHES the exclusion regex
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/excludedpaths/api/nested/path" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
        
        It "Should allow US IP on excluded /health path" {
            # /excludedpaths/health MATCHES the exclusion regex
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/excludedpaths/health" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
        
        It "Should still enrich country header on excluded paths" {
            # Even though blocking is skipped, GeoIP enrichment should still happen
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            
            # Use curl to see the response from whoami which echoes headers
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/excludedpaths/api/test") -join "`n"
            
            # The whoami service echoes back headers, so we should see X-Ipcountry: US
            $response | Should -Match "X-Ipcountry:\s*US"
        }
        
        It "Should block German IP on non-excluded path (not in allowed countries for this test)" {
            # German IP is normally allowed, but let's verify non-excluded paths are still processed
            # Actually DE is in allowedCountries for this endpoint, so it should be allowed
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/excludedpaths/somepath" -Headers $headers
            $result.StatusCode | Should -Be 200  # DE is in allowedCountries
        }
        
        It "Should block US IP on path that looks similar but doesn't match regex" {
            # /excludedpaths/apiversion does NOT match ^/excludedpaths/(api/.*|health)$
            # because it's "apiversion" not "api/"
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/excludedpaths/apiversion" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
        
        It "Should block US IP on /healthcheck (doesn't match /health exactly)" {
            # /excludedpaths/healthcheck does NOT match the regex (health vs healthcheck)
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/excludedpaths/healthcheck" -Headers $headers
            $result.StatusCode | Should -Be 403
        }
    }

    Context "Included Paths Regex" {
        # /includedpaths has includedPathsRegex=^[^/]*/includedpaths/secure/.*
        # and excludedPathsRegex=^[^/]*/includedpaths/secure/health$
        # Include first: only /secure/* can be blocked. Exclude still wins after include.

        It "Should allow US IP on a path that is not included" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/includedpaths/public" -Headers $headers
            $result.StatusCode | Should -Be 200
        }

        It "Should still enrich country header on a path that is not included" {
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/includedpaths/public") -join "`n"
            $response | Should -Match "X-Ipcountry:\s*US"
        }

        It "Should block US IP on an included path" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/includedpaths/secure/page" -Headers $headers
            $result.StatusCode | Should -Be 403
        }

        It "Should allow German IP on an included path" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/includedpaths/secure/page" -Headers $headers
            $result.StatusCode | Should -Be 200
        }

        It "Should allow US IP on an included path that is then excluded" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/includedpaths/secure/health" -Headers $headers
            $result.StatusCode | Should -Be 200
        }

        It "Should still enrich country header when exclude wins after include" {
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/includedpaths/secure/health") -join "`n"
            $response | Should -Match "X-Ipcountry:\s*US"
        }
    }

    Context "IPinfo Lite provider" {
        It "Should block US IP" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/ipinfo" -Headers $headers
            $result.StatusCode | Should -Be 403
        }

        It "Should allow German IP" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/ipinfo" -Headers $headers
            $result.StatusCode | Should -Be 200
        }

        It "Should enrich all IPinfo Lite fields on an allowed request" {
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.German_IP)" "$script:BaseUrl/ipinfo") -join "`n"
            $response | Should -Match "X-Ipcountry:\s*DE"
            $response | Should -Match "X-Geo-Country-Name:\s*Germany"
            $response | Should -Match "X-Geo-Continent:\s*Europe"
            $response | Should -Match "X-Geo-Continent-Code:\s*EU"
            $response | Should -Match "X-Geo-Isp:\s*Strato GmbH"
            $response | Should -Match "X-Geo-Domain:\s*strato\.de"
            $response | Should -Match "X-Geo-Asn:\s*AS6724"
            $response | Should -Match "X-Geo-Region:\s*null"
            $response | Should -Match "X-Geo-City:\s*null"
        }
    }

    Context "Merged BIN and IPinfo catalog sources" {
        It "Should write country from BIN LITE and ASN plus continent from IPinfo on one request" {
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/merged") -join "`n"
            $response | Should -Match "X-Ipcountry:\s*US"
            $response | Should -Match "X-Geo-Asn:\s*AS15169"
            $response | Should -Match "X-Geo-Continent:\s*North America"
        }
    }

    Context "MaxMind dummy Country provider" {
        It "Should block dummy CN IP" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.MaxMindDummyCN }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/maxmind" -Headers $headers
            $result.StatusCode | Should -Be 403
        }

        It "Should allow dummy GB IP" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.MaxMindDummyGB }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/maxmind" -Headers $headers
            $result.StatusCode | Should -Be 200
        }

        It "Should enrich GeoIP2 country fields on an allowed request" {
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.MaxMindDummyGB)" "$script:BaseUrl/maxmind") -join "`n"
            $response | Should -Match "X-Ipcountry:\s*GB"
            $response | Should -Match "X-Geo-Country-Name:\s*United Kingdom"
            $response | Should -Match "X-Geo-Continent:\s*Europe"
            $response | Should -Match "X-Geo-Continent-Code:\s*EU"
            $response | Should -Match "X-Geo-Asn:\s*null"
        }
    }

    Context "Token-protected database download (local .env)" {
        # Skipped in CI unless the matching token is set. Asserts Traefik plugin logs
        # only (never dumps logs — download URLs must not appear in test output).

        It "Should download and hot-swap the IP2Location token package" -Skip:(-not $script:HasIP2LocationToken) {
            $created = Wait-TraefikPluginLog -Pattern "created new database factory" -TimeoutSeconds 60
            $created | Should -BeTrue -Because "a new geo factory should be created for the token middleware"
            $downloaded = Wait-TraefikPluginLog -Pattern "database updated successfully.*_geo.BIN" -TimeoutSeconds 180
            if (-not $downloaded) {
                $zipFail = Wait-TraefikPluginLog -Pattern "failed to open zip file" -TimeoutSeconds 5
                $zipFail | Should -BeFalse -Because "a 200 HTML/error body is not a zip (log has file=/content_type/prefix; token is not logged)"
                $downloaded = Wait-TraefikPluginLog -Pattern "the available IP2Location database is not newer" -TimeoutSeconds 15
            }
            $downloaded | Should -BeTrue -Because "Traefik logs should show a token download or an already-current dated file (token value is not logged)"
            $swapped = Wait-TraefikPluginLog -Pattern "performHotSwap: database hot-swapped successfully" -TimeoutSeconds 60
            $swapped | Should -BeTrue -Because "Traefik logs should show a hot-swap after the token download"
        }

        It "Should download ASN LITE" -Skip:(-not $script:HasIP2LocationToken) {
            $asn = Wait-TraefikPluginLog -Pattern "database updated successfully.*_asn.BIN" -TimeoutSeconds 600
            $asn | Should -BeTrue -Because "Traefik logs should show the ASN LITE token download"
        }

        It "Should enrich DB8 region/city/isp and ASN on /tokendb" -Skip:(-not $script:HasIP2LocationToken) {
            $response = (curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/tokendb") -join "`n"
            $response | Should -Match "X-IP-Country:\s*US"
            $response | Should -Not -Match "X-Geo-Region:\s*null"
            $response | Should -Not -Match "X-Geo-City:\s*null"
            $response | Should -Not -Match "X-Geo-Isp:\s*null"
            $response | Should -Not -Match "X-Geo-Asn:\s*null"
        }

        It "Should download and open a current IPinfo Lite MMDB" -Skip:(-not $script:HasIPinfoToken) {
            $updated = Wait-TraefikPluginLog -Pattern "IPinfo database updated" -TimeoutSeconds 180
            if (-not $updated) {
                $updated = Wait-TraefikPluginLog -Pattern "the available IPinfo database is not newer" -TimeoutSeconds 15
            }
            $updated | Should -BeTrue -Because "Traefik logs should show an IPinfo token download or an already-current dated MMDB"
            $headers = @{ "X-Real-IP" = $script:TestIPs.German_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/ipinfo" -Headers $headers
            $result.StatusCode | Should -Be 200
        }
    }

    Context "Request header enrichment" {
        It "Should enrich country, region, and city request headers" {
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/enrichTest" -Headers $headers
            $result.StatusCode | Should -Be 200
            $result.Content | Should -Match "X-Geo-Country:\s*US"
            # LITE DB1 has no region/city/asn/isp/domain. Mapped headers are still written as null.
            $result.Content | Should -Not -Match "Please upgrade the data file"
            $result.Content | Should -Match "X-Geo-Region:\s*null"
            $result.Content | Should -Match "X-Geo-City:\s*null"
            $result.Content | Should -Match "X-Geo-Asn:\s*null"
            $result.Content | Should -Match "X-Geo-Isp:\s*null"
            $result.Content | Should -Match "X-Geo-Domain:\s*null"
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
    
    Context "Log Status Headers" {
        # Tests for logStatusDetailHeader
        # The /logheaders endpoint is configured with:
        # - logStatusDetailHeader: X-Geoblock-Decision (pass:reason or block:reason)
        # - allowedCountries: DE, AU
        # - blockedCountries: US
        # - ignoreVerbs: OPTIONS
        # - allowPrivate: true
        #
        # NOTE: These headers are added to the REQUEST for Traefik access logs,
        # NOT to the response. We verify them using Get-LastAccessLogEntry.
        
        It "Should log pass:allowed_country for AU IP in access log" {
            # 1.1.1.1 is an AU IP which is in allowedCountries
            curl -s -H "X-Real-IP: 1.1.1.1" "$script:BaseUrl/logheaders/test" | Out-Null
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "pass:allowed_country"
        }
        
        It "Should log pass:allowed_country for DE IP in access log" {
            # German IP is in allowedCountries
            curl -s -H "X-Real-IP: $($script:TestIPs.German_IP)" "$script:BaseUrl/logheaders/test" | Out-Null
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "pass:allowed_country"
        }
        
        It "Should log pass:allow_private for private IP in access log" {
            # Private IP with allowPrivate=true should pass
            curl -s -H "X-Real-IP: $($script:TestIPs.Private_IP)" "$script:BaseUrl/logheaders/test" | Out-Null
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "pass:allow_private"
        }
        
        It "Should log pass:ignore_verb for OPTIONS request in access log" {
            # OPTIONS verb is in ignoreVerbs, should pass even with blocked US IP
            curl -s -X OPTIONS -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/logheaders/test" | Out-Null
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "pass:ignore_verb"
        }
        
        It "Should log pass:excluded_regex for excluded path in access log" {
            # US IP is blocked, but /logheaders/api/* paths are excluded via excludedPathsRegex
            curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" "$script:BaseUrl/logheaders/api/endpoint" | Out-Null
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "pass:excluded_regex"
        }
        
        It "Should log pass:bypass_header when bypass header matches" {
            # US IP would be blocked, but bypass header allows it through
            curl -s -H "X-Real-IP: $($script:TestIPs.US_Google_DNS)" -H "X-Geoblock-Bypass: secret-bypass-token" "$script:BaseUrl/logheaders/test" | Out-Null
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "pass:bypass_header"
        }
        
        It "Should log block:default_allow for unlisted country when defaultAllow=false" {
            # Japanese IP is not in allowedCountries (DE, AU) or blockedCountries (US)
            # With defaultAllow=false, it should be blocked
            $headers = @{ "X-Real-IP" = $script:TestIPs.Japanese_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/logheaders/test" -Headers $headers
            $result.StatusCode | Should -Be 403
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "block:default_allow"
        }
        
        It "Should log block:blocked_ip_block for IP in blocked CIDR range" {
            # 3.5.140.1 is in the blockedIPBlocks CIDR range (3.5.140.0/24)
            $headers = @{ "X-Real-IP" = $script:TestIPs.AWS_Blocked_IP }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/logheaders/test" -Headers $headers
            $result.StatusCode | Should -Be 403
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "block:blocked_ip_block"
        }
        
        It "Should log block:blocked_country for US IP in access log" {
            # US is in blockedCountries - request should be blocked
            $headers = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }
            $result = Invoke-TestRequest -Uri "$script:BaseUrl/logheaders/test" -Headers $headers
            $result.StatusCode | Should -Be 403
            
            $logEntry = Get-LastAccessLogEntry
            $logEntry | Should -Not -BeNullOrEmpty
            $logEntry.'request_X-Geoblock-Decision' | Should -Be "block:blocked_country"
        }
    }

    Context "Shared middleware incarnation and config change" {
        # /reclaima /reclaimb /reclaimc share geoblock-reclaim. One plugin: key until
        # whoami-reclaim is recreated with a new country list (new hash, new put).
        # Default grace is 10s; the old key's reclaim_dispose must wait that long.

        It "Should put once for three routes, put again after config change, then dispose the old key after grace" {
            $repoRoot = Split-Path $PSScriptRoot -Parent
            $us = @{ "X-Real-IP" = $script:TestIPs.US_Google_DNS }

            foreach ($path in @("reclaima", "reclaimb", "reclaimc")) {
                $blocked = Invoke-TestRequest -Uri "$script:BaseUrl/$path" -Headers $us
                $blocked.StatusCode | Should -Be 403 -Because "$path should block US before the config change"
            }

            $firstPuts = Wait-ReclaimPluginPutCount -Count 1 -TimeoutSeconds 20
            $firstPuts.Count | Should -Be 1 -Because "three routes must share one plugin incarnation (one reclaim_put)"
            $oldKey = $firstPuts[0].Key
            $oldKey | Should -BeLike "plugin:geoblock-reclaim*"

            $env:RECLAIM_BLOCKED = "DE"
            $env:RECLAIM_ALLOWED = "US"
            Push-Location $repoRoot
            try {
                docker compose up -d --no-deps --force-recreate whoami-reclaim
                if ($LASTEXITCODE -ne 0) {
                    throw "docker compose failed to recreate whoami-reclaim"
                }
            }
            finally {
                Pop-Location
            }

            # Second put is the clock start: Traefik canceled the old New ctx in the same reload.
            $afterPuts = Wait-ReclaimPluginPutCount -Count 2 -TimeoutSeconds 30
            $afterPuts.Count | Should -Be 2 -Because "the new config hash must create a second plugin incarnation"
            $put2At = Get-Date
            $newKeys = @($afterPuts | ForEach-Object { $_.Key } | Select-Object -Unique)
            $newKeys.Count | Should -Be 2
            $newKey = @($newKeys | Where-Object { $_ -ne $oldKey })[0]
            $newKey | Should -Not -BeNullOrEmpty

            $earlyDispose = @(Get-ReclaimLogEvents | Where-Object { $_.Msg -eq 'reclaim_dispose' -and $_.Key -eq $oldKey })
            $earlyDispose.Count | Should -Be 0 -Because "grace is 10s; dispose must not run in the same reload as the new put"

            $switched = $false
            $elapsed = 0
            do {
                $probe = Invoke-TestRequest -Uri "$script:BaseUrl/reclaima" -Headers $us
                if ($probe.StatusCode -eq 200) {
                    $switched = $true
                    break
                }
                Start-Sleep 1
                $elapsed += 1
            } while ($elapsed -lt 20)
            $switched | Should -BeTrue -Because "after the label change, US must be allowed on /reclaima"

            foreach ($path in @("reclaimb", "reclaimc")) {
                $allowed = Invoke-TestRequest -Uri "$script:BaseUrl/$path" -Headers $us
                $allowed.StatusCode | Should -Be 200 -Because "$path shares the new middleware config"
            }

            $disposed = Wait-ReclaimDispose -Key $oldKey -TimeoutSeconds 15
            $disposed | Should -BeTrue -Because "reclaim_dispose for $oldKey should appear about 10s after the old holders were dropped"
            $graceSeconds = ((Get-Date) - $put2At).TotalSeconds
            $graceSeconds | Should -BeGreaterThan 8 -Because "default grace is 10s (saw dispose after $graceSeconds s)"

            $newStillLive = @(Get-ReclaimLogEvents | Where-Object { $_.Msg -eq 'reclaim_dispose' -and $_.Key -eq $newKey })
            $newStillLive.Count | Should -Be 0 -Because "the new incarnation must stay live"
        }
    }
} 