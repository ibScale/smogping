# SmogPing CLI Options

SmogPing supports several command-line options to control its behavior, configuration files, and output verbosity.

## Usage

```bash
./smogping [options]
```

## Command Line Options

### Configuration Control

| Option | Description | Default |
|--------|-------------|---------|
| `-config <file>` | Path to configuration file | `config.toml` |
| `-targets <file>` | Path to targets file | `targets.toml` |

### Output Control

| Option | Short | Description |
|--------|-------|-------------|
| `--verbose` | `-v` | Enable verbose output with additional operational details |
| `--debug` | `-d` | Enable debug output with detailed execution traces (implies verbose) |

### Feature Control

| Option | Description |
|--------|-------------|
| `--noalarm` | Disable the alarm system completely |
| `--nolog` | Disable alarm logging to syslog (startup/shutdown still logged) |

### Help

| Option | Description |
|--------|-------------|
| `--help` | Show usage information and available options |

## Output Levels

### Normal Mode (default)
- Single-line startup summary showing target count and stagger rate: `Monitoring 869 targets, starting 15 hosts/second`
- Error messages and warnings
- Alarm notifications
- Shutdown messages
- Batch flush notifications

### Verbose Mode (`--verbose` or `-v`)
- All normal mode output
- Configuration details: `[VERBOSE] Loaded configuration: InfluxDB=...`
- Organization listings: `Organization futurecloud: 26 hosts`
- Detailed target summary: `Total hosts to monitor: 869` and `Starting 15 hosts/second over 60 seconds`
- Ping cycle summaries per host
- Detailed timing information
- Batch processing details
- Alarm rate limiting messages
- Setup completion messages

### Debug Mode (`--debug` or `-d`)
- All verbose mode output
- Configuration file loading steps: `[DEBUG] Loading configuration files`
- Individual target details: `[DEBUG]   futurecloud-104-131-179-223 (104.131.179.223) - ping:250 loss:5 jitter:100`
- Individual ping attempts and results
- Data point calculations with RTT collections and statistics
- InfluxDB point creation details
- Alarm threshold checking
- Environment variable details for alarm scripts
- Individual host staggering information

## Examples

### Basic Usage
```bash
# Normal operation (uses config.toml and targets.toml)
./smogping

# Use custom configuration files
./smogping -config /etc/smogping/production.toml -targets /etc/smogping/prod-targets.toml

# With verbose output
./smogping --verbose

# With debug output (most detailed)
./smogping --debug
```

### Custom Configuration Files
```bash
# Development environment
./smogping -config configs/dev.toml -targets configs/dev-targets.toml

# Production with specific source monitoring
./smogping -config /opt/smogping/prod.toml -targets /opt/smogping/targets.toml -v

# Testing configuration
./smogping -config test.toml -targets test-targets.toml --debug
```

### Disable Alarms
```bash
# Run without alarm system
./smogping --noalarm

# Run with verbose output but no alarms
./smogping --verbose --noalarm

# Custom config without alarms
./smogping -config myconfig.toml -targets mytargets.toml --noalarm
```

### Testing and Troubleshooting
```bash
# Debug mode for troubleshooting
./smogping --debug

# Test specific configuration with debug output
./smogping -config test.toml -targets test-targets.toml --debug

# Run for a limited time with debug output
timeout 60s ./smogping --debug
```

## Log Format

- **Normal**: `2025/07/28 04:24:37 Monitoring 869 targets, starting 15 hosts/second`
- **Verbose**: `2025/07/28 04:17:47 [VERBOSE] Starting ping cycle for 869 targets`
- **Debug**: `2025/07/28 04:17:47 main.go:189: [DEBUG] Loading configuration files`

Debug mode includes file names and line numbers for easier troubleshooting.

## Configuration Files

SmogPing uses configurable files for flexibility:

### Default Files (if no options specified)
- `config.toml` - Main configuration file
- `targets.toml` - Target hosts to monitor 
- Included files specified in targets.toml (e.g., `vicihost.toml`)

### Custom Files (specified via command line)
```bash
# Use completely different configuration
./smogping -config /path/to/custom.toml -targets /path/to/custom-targets.toml

# Mix default config with custom targets
./smogping -targets special-targets.toml

# Use custom config with default targets
./smogping -config production.toml
```

### Configuration File Validation
SmogPing validates all configuration files on startup:
- **config.toml**: InfluxDB settings, ping parameters, source IP validation
- **targets.toml**: Organization structure, host definitions, per-target ping source IPs
- **Included files**: Additional target definitions with same validation rules

## Performance Considerations

- **Debug mode** generates significantly more log output and may impact performance with large numbers of targets
- **Verbose mode** provides a good balance of detail without excessive overhead
- **Normal mode** is recommended for production use
