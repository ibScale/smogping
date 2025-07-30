# SmogPing Source IP Configuration

SmogPing supports flexible source IP configuration for multi-homed systems, allowing you to control which network interface or IP address is used for outgoing ping packets.

## Overview

Source IP configuration operates on two levels:
1. **Global Configuration**: Set in `config.toml` applies to all hosts by default
2. **Per-Target Configuration**: Set in `targets.toml` overrides global setting for specific hosts

## Global Source IP Configuration

Configure a default source IP for all ping operations in your `config.toml`:

```toml
# Use specific source IP for all pings
ping_source = "192.168.1.100"

# Use default OS routing (same as omitting the field)
ping_source = "default"

# If not specified, defaults to "default"
# ping_source = 
```

### Global Source IP Values

| Value | Behavior |
|-------|----------|
| `"default"` | Let the operating system choose the best route |
| `"192.168.1.100"` | Use specific IP address as source |
| `""` (empty) | Same as "default" |
| Not specified | Same as "default" |

## Per-Target Source IP Configuration

Override the global setting for specific hosts in your `targets.toml`:

```toml
[organizations.example]
hosts = [
  # Use specific source IP for this host
  { name = "web-server", ip = "10.0.1.100", pingsource = "192.168.1.10" },
  
  # Use different source IP for this host
  { name = "db-server", ip = "10.0.2.100", pingsource = "192.168.2.10" },
  
  # Use global ping_source setting
  { name = "external-api", ip = "8.8.8.8" },
  
  # Explicitly use OS routing (overrides global setting)
  { name = "backup-server", ip = "10.0.3.100", pingsource = "default" },
]
```

### Per-Target Source IP Values

| Value | Behavior |
|-------|----------|
| `pingsource = "192.168.1.10"` | Use specific IP for this host only |
| `pingsource = "default"` | Use OS routing (overrides global setting) |
| `pingsource = ""` | Same as "default" |
| Not specified | Use global `ping_source` setting |

## Priority Order

Source IP selection follows this priority order:

1. **Per-target `pingsource`** (if specified and not "default")
2. **Global `ping_source`** (if specified and not "default")  
3. **Operating system routing** (default behavior)

## Use Cases

### Multi-Homed Servers

When your monitoring server has multiple network interfaces:

```toml
# config.toml - Default to management interface
ping_source = "192.168.100.10"

# targets.toml - Use specific interfaces for different networks
[organizations.datacenter]
hosts = [
  # DMZ hosts - use DMZ interface
  { name = "web1", ip = "10.1.1.100", pingsource = "10.1.1.10" },
  { name = "web2", ip = "10.1.1.101", pingsource = "10.1.1.10" },
  
  # Internal hosts - use internal interface  
  { name = "db1", ip = "10.2.1.100", pingsource = "10.2.1.10" },
  { name = "db2", ip = "10.2.1.101", pingsource = "10.2.1.10" },
  
  # External hosts - use internet interface
  { name = "dns", ip = "8.8.8.8", pingsource = "203.0.113.10" },
]
```

### Load Balancing and Routing

Distribute monitoring traffic across multiple interfaces:

```toml
[organizations.external]
hosts = [
  { name = "service1", ip = "1.1.1.1", pingsource = "203.0.113.10" },
  { name = "service2", ip = "8.8.8.8", pingsource = "203.0.113.11" },
  { name = "service3", ip = "8.8.4.4", pingsource = "203.0.113.12" },
]
```

### Testing and Debugging

Test connectivity from specific interfaces:

```toml
# Test same target from different source IPs
[organizations.connectivity_test]
hosts = [
  { name = "target-via-wan1", ip = "8.8.8.8", pingsource = "203.0.113.10" },
  { name = "target-via-wan2", ip = "8.8.8.8", pingsource = "203.0.113.20" },
  { name = "target-via-lan", ip = "8.8.8.8", pingsource = "192.168.1.100" },
]
```

## Validation

SmogPing validates source IP configurations on startup:

### Valid Source IP Values
- `"default"` - Always valid
- `"192.168.1.100"` - Valid IPv4 address
- `"2001:db8::1"` - Valid IPv6 address
- `""` (empty string) - Treated as "default"

### Invalid Source IP Values
- `"invalid-ip"` - Not a valid IP address
- `"hostname.example.com"` - Hostnames not supported
- `"192.168.1.256"` - Invalid IP address

### Error Messages

When validation fails, you'll see specific error messages:

```
TOML validation error in targets.toml: organizations.example.hosts[0].pingsource = invalid-ip - must be 'default' or a valid IP address
```

```
TOML validation error in config.toml: ping_source = not-an-ip - must be 'default' or a valid IP address
```

## InfluxDB Integration

The effective source IP is included in InfluxDB data points:

```json
{
  "measurement": "ping",
  "tags": {
    "host": "web-server",
    "ip": "10.0.1.100", 
    "organization": "production",
    "source": "192.168.1.10"  // Effective source IP used
  },
  "fields": {
    "rtt_avg": 25.5,
    "packet_loss": 0.0,
    "jitter": 1.2
  }
}
```

### Source Tag Values

| Scenario | Source Tag Value |
|----------|------------------|
| Per-target source IP used | Actual IP address (e.g., "192.168.1.10") |
| Global source IP used | Actual IP address (e.g., "192.168.1.100") |
| OS routing used | "default" |

## Troubleshooting

### Common Issues

1. **Permission Errors**: Raw socket operations may require elevated privileges
2. **Interface Not Available**: Source IP must be assigned to a local interface
3. **Routing Issues**: Source IP must be able to reach the target network

### Debug Information

Use verbose or debug mode to see source IP selection:

```bash
./smogping --debug
```

Debug output shows source IP decisions:
```
[VERBOSE] Using source IP 192.168.1.100 for pinging 8.8.8.8
```

### Testing Source IP Configuration

Test your source IP configuration:

```bash
# Test with minimal targets
./smogping -targets test-targets.toml --verbose

# Debug source IP selection
./smogping --debug | grep "source IP"
```

## Best Practices

1. **Document Your Setup**: Comment your source IP choices in configuration files
2. **Test Connectivity**: Verify each source IP can reach its intended targets
3. **Monitor Interface Status**: Ensure source interfaces remain available
4. **Use Consistent Naming**: Use descriptive names for hosts with specific source IPs
5. **Group by Network**: Organize targets by network/interface for easier management

## Performance Considerations

- Source IP configuration has minimal performance impact
- Per-target source IPs are cached and reused efficiently
- No additional DNS lookups are performed for source IP addresses
- Source IP validation occurs only during configuration loading
