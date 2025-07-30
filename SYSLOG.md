# SmogPing Syslog Integration

SmogPing integrates with the system's syslog daemon to provide structured logging for critical events. This allows monitoring and log aggregation systems to easily track SmogPing's operational status and alerts.

## What Gets Logged to Syslog

### 1. Startup Summary (INFO level)
When SmogPing starts, it logs a summary of the monitoring configuration:
```
SmogPing started: monitoring 869 targets, starting 15 hosts/second over 60 seconds
```

### 2. Alarm Events (WARNING level)
When network alarm thresholds are exceeded:
```
ALARM: example-host (192.168.1.1) in myorg - ping_time=250.5ms>200ms - RTT=250.5ms LOSS=0.0% JITTER=15.2ms
```

## Controlling Syslog Output

### Disabling Alarm Logging
Use the `--nolog` flag to disable alarm logging to syslog while keeping startup and shutdown messages:
```bash
./smogping --nolog
```

This is useful when:
- You want alarm notifications through scripts only
- You're using external log aggregation for alarms
- You want to reduce syslog volume while maintaining operational visibility

**Note**: Startup and shutdown messages will still be logged to syslog for operational monitoring.

## Systemd Journal Integration

When running as a systemd service, all syslog messages are automatically captured by journalctl.

### Viewing SmogPing Logs
```bash
# View all SmogPing logs
journalctl -t smogping

# View recent logs
journalctl -t smogping -f

# View logs from the last hour
journalctl -t smogping --since "1 hour ago"

# View only alarm messages
journalctl -t smogping -p warning

# View startup messages
journalctl -t smogping -p info
```

### Service Configuration
The systemd service file includes the following syslog configuration:
```ini
[Service]
StandardOutput=journal
StandardError=journal
SyslogIdentifier=smogping
```

This ensures:
- All stdout/stderr goes to the journal
- Messages are tagged with "smogping" identifier
- Both regular log output and syslog messages are captured

## Log Message Format

### Startup Messages
- **Level**: INFO
- **Format**: `SmogPing started: monitoring {count} targets, starting {rate} hosts/second over {duration} seconds`
- **Example**: `SmogPing started: monitoring 869 targets, starting 15 hosts/second over 60 seconds`

### Alarm Messages
- **Level**: WARNING
- **Format**: `ALARM: {hostname} ({ip}) in {organization} - {reasons} - RTT={rtt}ms LOSS={loss}% JITTER={jitter}ms`
- **Example**: `ALARM: web-server-01 (10.0.1.100) in production - ping_time=350.2ms>300ms,packet_loss=2.5%>2% - RTT=350.2ms LOSS=2.5% JITTER=45.8ms`

## Log Aggregation Integration

### Syslog-ng Configuration
```conf
destination d_smogping {
    file("/var/log/smogping.log");
};

filter f_smogping {
    program("smogping");
};

log {
    source(s_src);
    filter(f_smogping);
    destination(d_smogping);
};
```

### rsyslog Configuration
```conf
:programname, isequal, "smogping" /var/log/smogping.log
& stop
```

### Fluentd Configuration
```conf
<source>
  @type systemd
  tag systemd.smogping
  matches [{ "_SYSTEMD_UNIT": "smogping.service" }]
  read_from_head true
</source>

<filter systemd.smogping>
  @type parser
  key_name MESSAGE
  <parse>
    @type regexp
    expression /^(?<level>\w+): (?<message>.*)$/
  </parse>
</filter>
```

## Alerting Integration

### Prometheus + Alertmanager
Use a log-to-metrics exporter to convert syslog messages to Prometheus metrics:

```yaml
# promtail config
- job_name: journal
  journal:
    json: false
    max_age: 12h
    labels:
      job: systemd-journal
  pipeline_stages:
  - match:
      selector: '{unit="smogping.service"}'
      stages:
      - regex:
          expression: 'ALARM: (?P<host>\S+) \((?P<ip>\S+)\) in (?P<org>\S+) - (?P<reasons>.*) - RTT=(?P<rtt>\S+) LOSS=(?P<loss>\S+) JITTER=(?P<jitter>\S+)'
      - metrics:
          smogping_alarm_total:
            type: Counter
            description: "Total number of SmogPing alarms"
            config:
              action: inc
              match_all: true
```

### Grafana Loki Query Examples
```logql
# All SmogPing logs
{unit="smogping.service"}

# Only alarm messages
{unit="smogping.service"} |= "ALARM:"

# Alarms for specific organization
{unit="smogping.service"} |= "ALARM:" |= "production"

# High RTT alarms
{unit="smogping.service"} |= "ping_time=" | regexp "RTT=(?P<rtt>[0-9.]+)ms" | rtt > 500
```

## Monitoring and Health Checks

### Service Health Check
```bash
#!/bin/bash
# Check if SmogPing service is healthy
if systemctl is-active --quiet smogping; then
    # Check for recent startup message
    if journalctl -t smogping --since "10 minutes ago" -p info | grep -q "SmogPing started"; then
        echo "SmogPing is healthy"
        exit 0
    fi
fi
echo "SmogPing health check failed"
exit 1
```

### Alarm Rate Monitoring
```bash
#!/bin/bash
# Monitor alarm rate
ALARM_COUNT=$(journalctl -t smogping --since "1 hour ago" -p warning | grep -c "ALARM:")
if [ "$ALARM_COUNT" -gt 10 ]; then
    echo "High alarm rate: $ALARM_COUNT alarms in last hour"
    exit 1
fi
```

## Security Considerations

- Syslog messages may contain IP addresses and hostnames
- Consider log rotation and retention policies
- Ensure appropriate permissions on log files
- Use secure transport for remote syslog if needed

## Troubleshooting

### Syslog Not Working
```bash
# Check if syslog daemon is running
systemctl status rsyslog
# or
systemctl status syslog-ng

# Test syslog functionality
logger -t smogping "Test message"
journalctl -t smogping --since "1 minute ago"
```

### Missing Messages
- Check syslog daemon configuration
- Verify systemd service has correct SyslogIdentifier
- Check for syslog filtering rules that might drop messages

### Permission Issues
- Ensure SmogPing user has permission to write to syslog socket
- Check SELinux/AppArmor policies if applicable
