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