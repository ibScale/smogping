# SmogPing File Watching

SmogPing automatically monitors target configuration files for changes and reloads them without requiring a restart. This allows for dynamic target management with minimal disruption to ongoing monitoring operations.

## Monitored Files

SmogPing watches the following files for changes:

- **targets.toml** - Target hosts configuration
- **All included files** - Files specified in `targets.toml` include array (e.g., `vicihost.toml`)

**Note**: Configuration files (`config.default.toml` and `config.toml`) are **not** monitored as these contain static operational parameters that should not change during runtime.

## How It Works

### File Change Detection
- Uses `fsnotify` for efficient file system monitoring
- Detects `WRITE` and `CREATE` events on monitored files
- Implements a 2-second debounce to prevent multiple rapid reloads

### Reload Process
1. **Target Reload**: Reloads targets from `targets.toml` and included files
2. **Target Comparison**: Compares old and new targets to identify changes
3. **Minimal Disruption**: Only affects changed targets, leaving unchanged targets running
4. **Error Handling**: Keeps current targets if reload fails

### Target Change Detection
The system identifies three types of target changes:
- **Added**: New targets that weren't in the previous configuration
- **Removed**: Targets that were removed from the configuration
- **Unchanged**: Targets that remain the same (these continue uninterrupted)

## Logging

### Console Output
File watching events are logged with different verbosity levels:

```bash
# Normal mode
Target changes detected: 5 added, 2 removed, 862 unchanged

# Verbose mode
[VERBOSE] Target file changed: targets.toml
[VERBOSE] Reloading targets...
[VERBOSE] Now watching new included file: newfile.toml
Added targets:
  web-server-05 (10.0.1.105) in production
  api-server-03 (10.0.2.103) in staging
Removed targets:
  old-server-01 (10.0.3.101) in legacy
```

### Syslog Integration
Target changes are logged to syslog for monitoring:

```bash
# Target changes
SmogPing: Targets reloaded: monitoring 867 targets, starting 15 hosts/second over 60 seconds
```

View in journalctl:
```bash
journalctl -t smogping -p info
```

## Benefits

### Zero Downtime Updates
- **No service restart required** for target changes
- **Existing monitoring continues** uninterrupted for unchanged targets
- **Immediate effect** for new targets (added within next monitoring cycle)

### Dynamic Scaling
- **Add new hosts** by editing `targets.toml` or included files
- **Remove decommissioned hosts** without affecting others
- **Update alarm thresholds** for specific hosts
- **Modify organization structure** for better management

### Operational Efficiency
- **Real-time target management** without downtime
- **Automatic inclusion** of new files added to include list
- **Error recovery** maintains service if target configuration is invalid

## Target Configuration Examples

### Adding New Targets
Edit `targets.toml` or any included file:
```toml
[organizations.production]
hosts = [
    { name = "web-server-01", ip = "10.0.1.101", alarmping = 200, alarmloss = 2, alarmjitter = 50 },
    { name = "web-server-02", ip = "10.0.1.102", alarmping = 200, alarmloss = 2, alarmjitter = 50 },
    # New server added here
    { name = "web-server-03", ip = "10.0.1.103", alarmping = 200, alarmloss = 2, alarmjitter = 50 },
]
```

SmogPing will detect the change and start monitoring `web-server-03` within 2-3 seconds.

### Including New Files
Add new file to include list in `targets.toml`:
```toml
include = ["vicihost.toml", "newdatacenter.toml"]
```

SmogPing will automatically start watching `newdatacenter.toml` for future changes.

## Error Handling

### Invalid Target Configuration
If a target file contains syntax errors:
```bash
Error reloading targets: failed to load targets: Near line 5: expected key separator '=' - keeping current targets
```

The system continues operating with the previous valid target configuration.

### Missing Files
If an included file is deleted:
```bash
Warning: failed to load included file missing.toml: no such file or directory
```

The system continues with remaining valid files.

### File Permission Issues
If SmogPing cannot watch a file due to permissions:
```bash
Warning: Failed to watch file targets.toml: permission denied
```

The file won't be monitored for changes, but the system continues operating.

## Monitoring File Watching

### Verify Watched Files
Enable verbose mode to see which files are being watched:
```bash
./smogping --verbose
```

Look for output like:
```
[VERBOSE] Watching file: targets.toml
[VERBOSE] Watching file: vicihost.toml
[VERBOSE] File watching configured for target changes
```

### Test Target Reload
1. Start SmogPing with verbose output
2. Make a small change to a target file
3. Observe reload messages in the logs
4. Verify target counts and changes are detected correctly

### Monitor via Syslog
Use journalctl to monitor target changes:
```bash
# Watch for target changes
journalctl -t smogping -f | grep -E "(reloaded|Target)"

# View recent target events
journalctl -t smogping --since "1 hour ago" -p info
```

## Best Practices

### Target Management
- **Test target syntax** before saving files
- **Use atomic file operations** (save to temp file, then move) for large changes
- **Monitor logs** after making changes to verify successful reload
- **Keep backups** of working target configurations

### Performance Considerations
- **Large target changes** may cause brief monitoring gaps during reload
- **Frequent file changes** are debounced to prevent excessive reloads
- **File watching overhead** is minimal for normal operations

### Troubleshooting
- **Check file permissions** if watching fails
- **Verify file syntax** if reload fails
- **Monitor system resources** during large target changes
- **Use debug mode** to see detailed file watching events

This file watching feature enables true dynamic target management for SmogPing, allowing operational teams to manage network monitoring targets without service interruptions.
