# SmogPing - Network Monitoring Tool

SmogPing is a Go application that monitors network quality by pinging targets and storing metrics in an InfluxDB v2 bucket. It measure RTT (Round Trip Time), Jitter, and Packet Loss with a color coded graphing dashboard.

It's a spiritual successor to the venerable SmokePing. In a world of increasing containerization with cloud backed storage and locked down VMs the reality of running SmokePing as a PERL cgi-bin web application with flat-file RRD databases and an external fping syscall dependancy has become more and more of a challenge. That coupled with scaling issues that start to happen when you get to 1000+ targets is what led to the development of SmogPing.

Props to Tobias Oetiker and the OG [SmokePing](https://github.com/oetiker/SmokePing)

## Features

- **Configuration Management**: Loads settings from configurable files with comprehensive validation
- **Target Management**: Reads ping targets from configurable files with support for included files
- **TOML Validation**: Comprehensive validation with detailed error messages and context
- **Dynamic Reload**: Monitors configuration files for changes and reloads without restart
- **Network Monitoring**: Uses `github.com/prometheus-community/pro-bing` for modern ping operations
- **Source IP Control**: Global and per-target source IP configuration for multi-homed systems
- **Metrics Collection**: Calculates average ping time, packet loss, and jitter
- **InfluxDB Integration**: Stores metrics in InfluxDB v2 with configurable batching
- **DNS Support**: Automatic hostname resolution with periodic refresh monitoring
- **Individual Ping Schedules**: Each target runs on its own independent schedule with staggered starts
- **Alarm System**: Configurable thresholds with script-based alerting and receiver filtering
- **Syslog Integration**: Logs startup summary and alarms to system journal
- **Graceful Shutdown**: Handles SIGINT/SIGTERM for clean shutdown
- **Configuration Validation**: Comprehensive sanity checks ensure viable monitoring setup
- **Performance Optimization**: Object pooling, rate limiting, and concurrency control

## Available Documentation

- **[CLI.md](CLI.md)**: Command-line interface options and configuration files
- **[SOURCE_IP.md](SOURCE_IP.md)**: Source IP configuration for multi-homed systems
- **[ALARMS.md](ALARMS.md)**: Alarm system configuration and operation
- **[BATCHING.md](BATCHING.md)**: InfluxDB batching configuration and optimization
- **[OPTIMIZATIONS.md](OPTIMIZATIONS.md)**: Performance tuning and optimization
- **[OBJECT_POOL.md](OBJECT_POOL.md)**: Object pooling and individual ping schedule architecture
- **[VALIDATION.md](VALIDATION.md)**: Configuration validation and troubleshooting
- **[SYSLOG.md](SYSLOG.md)**: System logging and journalctl integration
- **[FILE_WATCHING.md](FILE_WATCHING.md)**: Dynamic configuration reloading without restart
- **[DNS_SUPPORT.md](DNS_SUPPORT.md)**: DNS hostname resolution and monitoring

## Configuration

SmogPing uses a simplified configuration system with two main files:

### config.toml (Main Configuration)
Contains your actual configuration settings including InfluxDB connection details and global ping settings.

Example:
```toml
# InfluxDB connection
influx_url = "http://localhost:8086"
influx_token = "your-actual-token"
influx_org = "your-org"
influx_bucket = "your-bucket"

# Ping configuration
data_point_pings = 5
data_point_time = 60
ping_timeout = 5
ping_source = "192.168.1.100"  # Global source IP (optional)

# System limits (used for capacity validation)
max_concurrent_pings = 50

# DNS and alarm settings
dns_refresh = 600
alarm_rate = 300
alarm_receiver = "./alarmreceiver.sh"
```

### targets.toml (Target Configuration)
Defines the targets to monitor, organized by organizations. Supports including additional files and per-target ping source configuration.

Example:
```toml
include = ["vicihost.toml"]

[organizations]

[organizations.production]
hosts = [
  { name = "web-server", ip = "10.0.1.100", pingsource = "10.0.1.10" },
  { name = "db-server", ip = "10.0.2.100", pingsource = "10.0.2.10" },
  { name = "external-service", ip = "8.8.8.8" },  # Uses global ping_source
]

[organizations.monitoring]
hosts = [
  { name = "dns-primary", ip = "1.1.1.1", alarmping = 100, alarmloss = 5, alarmreceiver = "./dns-alarm.sh" },
  { name = "dns-secondary", ip = "8.8.4.4", pingsource = "default" },  # Uses OS routing
]
```

## Usage

1. **Configure SmogPing**: Create `config.toml` with your InfluxDB connection details
2. **Set up targets**: Create `targets.toml` with your ping targets and any included files
3. **Run the application**:
   ```bash
   # Basic usage (uses config.toml and targets.toml)
   ./smogping
   
   # Use custom configuration files
   ./smogping -config /path/to/myconfig.toml -targets /path/to/mytargets.toml
   
   # Using short options
   ./smogping -c /path/to/myconfig.toml -t /path/to/mytargets.toml
   
   # With verbose output
   ./smogping --verbose
   
   # With debug output (detailed troubleshooting)
   ./smogping --debug
   
   # Disable alarm system
   ./smogping --noalarm
   
   # Disable syslog logging
   ./smogping --nolog
   
   # Show help
   ./smogping --help
   ```

### Command Line Options

- `-config <file>`, `-c <file>`: Specify configuration file (default: config.toml)
- `-targets <file>`, `-t <file>`: Specify targets file (default: targets.toml)
- `--verbose, -v`: Enable verbose output
- `---debug, -d`: Enable debug output
- `--noalarm`: Disable alarm system
- `--nolog`: Disable alarm logging to syslog

## Source IP Configuration

SmogPing supports flexible source IP configuration for multi-homed systems:

### Global Source IP
Set in `config.toml`:
```toml
ping_source = "192.168.1.100"  # All pings use this source IP
```

### Per-Target Source IP
Override in `targets.toml` for specific hosts:
```toml
[organizations.example]
hosts = [
  { name = "host1", ip = "8.8.8.8", pingsource = "192.168.1.100" },  # Uses specific source
  { name = "host2", ip = "8.8.4.4", pingsource = "192.168.2.100" },  # Uses different source
  { name = "host3", ip = "1.1.1.1" },                                 # Uses global ping_source
  { name = "host4", ip = "1.0.0.1", pingsource = "default" },        # Uses OS routing
]
```

**Priority**: Per-target `pingsource` > Global `ping_source` > OS routing

## Building

```bash
cd /directory/to/smogping
go build -o smogping main.go
ln -s /directory/to/smogping/webapp /srv/www/htdocs/smogping

```

## Dependencies

- `github.com/BurntSushi/toml` - TOML configuration parsing
- `github.com/prometheus-community/pro-bing` - Modern ICMP ping functionality
- `github.com/influxdata/influxdb-client-go/v2` - InfluxDB v2 client
- `github.com/fsnotify/fsnotify` - File system monitoring for config reloading

## Data Points

The application writes the following metrics to InfluxDB:

- **Measurement**: `ping`
- **Tags**:
  - `host`: Target host name
  - `ip`: Target IP address or hostname
  - `organization`: Organization name from targets config
  - `source`: Effective source IP used for ping ("default" if OS routing)
  - `resolved_ip`: Actual resolved IP (if different from ip tag)
  - `is_dns_name`: "true" if target was a hostname, "false" if IP
- **Fields**:
  - `rtt_avg`: Average round-trip time in milliseconds
  - `packet_loss`: Packet loss percentage
  - `jitter`: Jitter (standard deviation of RTTs) in milliseconds

## Alarm System

SmogPing includes a flexible alarm system with intelligent receiver filtering:

### Global Alarm Configuration
```toml
alarm_rate = 300           # Minimum seconds between alarms for same host
alarm_receiver = "./alarmreceiver.sh"  # Default alarm script
```

### Per-Target Alarm Configuration
```toml
[organizations.example]
hosts = [
  { name = "critical-server", ip = "10.0.1.100", 
    alarmping = 100, alarmloss = 5, alarmjitter = 50,
    alarmreceiver = "./critical-alarm.sh" },
  { name = "non-critical", ip = "10.0.2.100", alarmreceiver = "none" },  # No alarms
]
```

**Alarm Receiver Filtering**:
- Hosts with no `alarmreceiver` defined skip alarm checks entirely
- Hosts with `alarmreceiver = "none"` skip alarm checks
- Improves performance by avoiding unnecessary alarm processing

## Monitoring Interval

Each target runs on its own independent schedule, sending `data_point_pings` pings (default: 5) every `data_point_time` seconds (default: 60 seconds). Targets use staggered start times to distribute load evenly over the monitoring interval.

## Graceful Shutdown

The application responds to SIGINT (Ctrl+C) and SIGTERM signals for graceful shutdown, ensuring all pending operations complete before exiting.

## Configuration Validation

SmogPing performs comprehensive validation on startup:

- **Capacity checking**: Ensures target count doesn't exceed system limits
- **Timing validation**: Verifies ping intervals and timeouts are sensible  
- **Rate limit verification**: Confirms all pings can complete within time windows
- **InfluxDB settings**: Validates batch configuration for optimal performance

See `VALIDATION.md` for detailed information about configuration validation and capacity planning.

## InfluxDB Batching

SmogPing implements intelligent batching for optimal InfluxDB performance:

- **Size-based flushing**: Writes batch when `influx_batch_size` points collected
- **Time-based flushing**: Ensures data written within `influx_batch_time` seconds  
- **Graceful shutdown**: Final flush prevents data loss on exit
- **Thread-safe**: Safe concurrent access from individual target goroutines

See `BATCHING.md` for detailed information about batching configuration and performance tuning.
