# SmogPing DNS Support Implementation

## 🌐 **DNS Resolution Features**

### **1. Automatic DNS Detection**
**Purpose**: SmogPing automatically detects whether a target is an IP address or DNS hostname

**Implementation**:
- **IP Address Detection**: Uses `net.ParseIP()` to validate IP addresses
- **DNS Name Detection**: Identifies hostnames containing dots and letters
- **Mixed Support**: Handles both IP addresses and DNS names in the same configuration

**Examples**:
```toml
[organizations.MyOrg]
hosts = [
    { name = "Server1", ip = "192.168.1.100" },      # IP address - used directly
    { name = "Server2", ip = "webserver.company.com" }, # DNS name - resolved automatically
    { name = "Google", ip = "google.com" },          # DNS name - resolved to IP
    { name = "Cloudflare", ip = "1.1.1.1" }         # IP address - used directly
]
```

### **2. DNS Pre-flight Checks**
**Purpose**: Resolve all DNS names during application startup to validate configuration

**Process**:
1. **Scan all targets** for DNS names vs IP addresses
2. **Resolve DNS names** to IP addresses with timeout (5 seconds)
3. **Cache resolutions** for future use
4. **Validate connectivity** ensuring all targets are reachable
5. **Report summary** of resolution results

**Output Example**:
```
[VERBOSE] Performing DNS pre-flight checks...
[DEBUG] Host Google DNS (8.8.8.8) in DNSTest: IP address detected
[DEBUG] Host Google Main (google.com) in DNSTest: DNS name detected
[VERBOSE] Resolved google.com -> 142.251.15.100 for host Google Main in DNSTest
[VERBOSE] DNS pre-flight checks completed: 2 DNS names, 2 IP addresses, 0 errors
```

### **3. Periodic DNS Refresh**
**Purpose**: Monitor DNS names for IP address changes and update automatically

**Configuration**:
```toml
# How long in seconds to look for DNS IP changes
dns_refresh = 600  # Default: 10 minutes
```

**Features**:
- **Configurable interval**: Set refresh frequency via `dns_refresh` setting
- **Change detection**: Automatically detects when DNS resolves to different IP
- **Minimal disruption**: Only updates changed targets, preserves unchanged ones
- **Comprehensive logging**: Logs all DNS changes to console and syslog

**DNS Change Example**:
```
DNS CHANGE: webserver01 (webserver.company.com) in MyOrg changed from 192.168.1.100 to 192.168.1.101
```

### **4. Enhanced Data Storage**
**Purpose**: Store both original DNS names and resolved IPs in InfluxDB

**InfluxDB Tags**:
```
host: "webserver01"                    # Host name
ip: "webserver.company.com"           # Original DNS name or IP
resolved_ip: "192.168.1.100"          # Resolved IP (if DNS name)
is_dns_name: "true"                   # Whether target is DNS name
organization: "MyOrg"                 # Organization
source: "Main"                        # Ping source
```

**Benefits**:
- **Complete traceability**: Track both DNS name and resolved IP
- **Historical analysis**: See DNS changes over time
- **Flexible querying**: Query by DNS name or resolved IP
- **Change tracking**: Identify patterns in DNS changes

## 🔧 **Configuration**

### **DNS Refresh Settings**:
```toml
# DNS refresh interval in seconds
dns_refresh = 600     # 10 minutes (default)
dns_refresh = 300     # 5 minutes (more frequent)
dns_refresh = 1800    # 30 minutes (less frequent)
dns_refresh = 0       # Disable DNS refresh monitoring
```

### **Recommended Settings by Environment**:

**Production Environment**:
```toml
dns_refresh = 600     # 10 minutes - good balance
```

**Development Environment**:
```toml
dns_refresh = 300     # 5 minutes - faster change detection
```

**Stable Environment**:
```toml
dns_refresh = 1800    # 30 minutes - minimal overhead
```

**DNS-Free Environment**:
```toml
dns_refresh = 0       # Disabled - only IP addresses used
```

## 📊 **DNS Resolution Process**

### **1. Pre-flight Resolution**:
```
Application Startup
        │
        ▼
┌─────────────────┐
│  Load Targets   │
└─────────────────┘
        │
        ▼
┌─────────────────┐     ┌─────────────────┐
│  Detect DNS vs  │────▶│   Resolve DNS   │
│   IP Addresses  │     │   Names to IPs  │
└─────────────────┘     └─────────────────┘
        │                        │
        ▼                        ▼
┌─────────────────┐     ┌─────────────────┐
│  Cache Results  │◀────│  Validate All   │
│   in Memory     │     │   Resolutions   │
└─────────────────┘     └─────────────────┘
```

### **2. Ping Execution**:
```
Ping Job Created
        │
        ▼
┌─────────────────┐
│  Check Target   │
│      Type       │
└─────────────────┘
        │
        ▼
   ┌─────────┐     ┌─────────────────┐
   │IP Addr? │────▶│ Use IP Directly │
   └─────────┘     └─────────────────┘
        │ No
        ▼ DNS Name
┌─────────────────┐     ┌─────────────────┐
│ Use Resolved IP │────▶│  Execute Ping   │
│  from Cache     │     │  to Target IP   │
└─────────────────┘     └─────────────────┘
```

### **3. DNS Refresh Cycle**:
```
Timer Triggered (dns_refresh interval)
        │
        ▼
┌─────────────────┐
│ Scan All DNS    │
│     Targets     │
└─────────────────┘
        │
        ▼
┌─────────────────┐     ┌─────────────────┐
│ Resolve Current │────▶│  Compare with   │
│   IP Addresses  │     │  Cached IPs     │
└─────────────────┘     └─────────────────┘
        │                        │
        ▼                        ▼
┌─────────────────┐     ┌─────────────────┐
│ Update Cache &  │◀────│   Log Changes   │
│ Target Config   │     │  to Syslog      │
└─────────────────┘     └─────────────────┘
```

## 🛡️ **Error Handling**

### **DNS Resolution Failures**:
- **Timeout Protection**: 5-second timeout for DNS lookups
- **Graceful Degradation**: Failed resolutions don't stop application startup
- **Error Reporting**: Clear error messages with hostname and organization context
- **Retry Logic**: DNS refresh will retry failed resolutions on next cycle

### **Network Connectivity Issues**:
- **Context Cancellation**: Respects application shutdown during long DNS operations
- **Custom Resolver**: Configured with appropriate timeouts and Go DNS resolver
- **IPv4 Preference**: Prioritizes IPv4 addresses for ping compatibility

### **Configuration Validation**:
- **Pre-flight Validation**: All DNS names must resolve during startup
- **Startup Failure**: Application fails to start if critical DNS resolutions fail
- **Warning System**: Non-critical DNS failures generate warnings but allow startup

## 📈 **Performance Impact**

### **Memory Usage**:
- **DNS Cache**: Minimal memory overhead (~50 bytes per DNS name)
- **Resolution History**: Tracks DNS changes but doesn't store full history
- **Efficient Storage**: Uses existing target structures with additional fields

### **Network Overhead**:
- **Pre-flight Phase**: One DNS lookup per unique hostname at startup
- **Refresh Phase**: One DNS lookup per unique hostname every `dns_refresh` seconds
- **Optimized Timing**: DNS refresh separate from ping operations

### **Typical Performance**:
```
DNS Names: 50 hostnames
Memory: ~2.5KB for DNS cache
Network: 50 DNS queries every 600 seconds (10 minutes)
Latency: Minimal impact on ping operations
```

## 🎯 **Use Cases**

### **1. Load Balanced Services**:
```toml
[organizations.WebServices]
hosts = [
    { name = "API Server", ip = "api.mycompany.com" },      # May change IPs
    { name = "Web Server", ip = "www.mycompany.com" },      # Load balanced
    { name = "Database", ip = "db.mycompany.com" }          # High availability
]
```

### **2. Cloud Infrastructure**:
```toml
[organizations.AWS]
hosts = [
    { name = "ELB Frontend", ip = "frontend-123456.us-west-2.elb.amazonaws.com" },
    { name = "RDS Endpoint", ip = "mydb.cluster-xyz.us-west-2.rds.amazonaws.com" }
]
```

### **3. CDN Monitoring**:
```toml
[organizations.CDN]
hosts = [
    { name = "Cloudflare Edge", ip = "mysite.cloudflare.com" },
    { name = "AWS CloudFront", ip = "d1234567890.cloudfront.net" }
]
```

### **4. Mixed Environment**:
```toml
[organizations.Infrastructure]
hosts = [
    { name = "Gateway", ip = "10.0.1.1" },                 # Static IP
    { name = "Load Balancer", ip = "lb.internal.com" },    # DNS name
    { name = "Backup Server", ip = "192.168.1.100" },      # Static IP
    { name = "API Endpoint", ip = "api.external.com" }     # DNS name
]
```

## 🔍 **Monitoring and Debugging**

### **Verbose Output**:
```bash
./smogping --verbose
# Shows DNS resolution details and refresh activity
```

### **Debug Output**:
```bash
./smogping --debug  
# Shows detailed DNS detection and resolution process
```

### **Syslog Integration**:
- **DNS Changes**: Logged as warnings to syslog
- **Resolution Summary**: Startup resolution summary logged as info
- **Error Tracking**: DNS resolution failures logged appropriately

### **InfluxDB Queries**:
```sql
-- Monitor DNS changes
SELECT * FROM ping WHERE is_dns_name='true' AND resolved_ip != ip

-- Track DNS performance
SELECT mean(rtt_avg) FROM ping WHERE host='webserver01' GROUP BY resolved_ip

-- DNS change frequency
SELECT count(*) FROM ping WHERE is_dns_name='true' GROUP BY resolved_ip, time(1h)
```

## 🚀 **Benefits**

### **Operational**:
- ✅ **Automatic adaptation** to infrastructure changes
- ✅ **No manual IP updates** required when services move
- ✅ **Complete visibility** into DNS changes and their impact
- ✅ **Seamless monitoring** across cloud and on-premise environments

### **Technical**:
- ✅ **Zero-downtime DNS updates** - changes applied dynamically
- ✅ **Historical tracking** of DNS resolutions in InfluxDB
- ✅ **Performance monitoring** of DNS-based vs IP-based targets
- ✅ **Robust error handling** for DNS resolution failures

### **Administrative**:
- ✅ **Simplified configuration** - use meaningful DNS names
- ✅ **Reduced maintenance** - automatic handling of IP changes  
- ✅ **Better documentation** - DNS names are self-documenting
- ✅ **Consistent monitoring** regardless of underlying IP changes

The DNS support transforms SmogPing from a static IP monitoring tool into a dynamic, cloud-ready network monitoring solution that automatically adapts to modern infrastructure patterns.
