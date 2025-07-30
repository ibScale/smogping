# SmogPing Alarm System

## 🚨 **Alarm Overview**

SmogPing monitors ping results against configurable thresholds and executes alarm receiver scripts when limits are exceeded.

## ⚙️ **Alarm Configuration**

### **Per-Host Alarm Thresholds**
Configure alarm thresholds in your target files:

```toml
[organizations.example]
hosts = [
  { 
    name = "Critical Server", 
    ip = "192.168.1.10", 
    alarmping = 250,           # RTT threshold in milliseconds
    alarmloss = 5,             # Packet loss threshold in percentage
    alarmjitter = 100,         # Jitter threshold in milliseconds
    alarmreceiver = "./custom-alarm.sh"  # Optional: custom alarm script
  }
]
```

### **Global Alarm Settings**
In `config.default.toml` or `config.toml`:

```toml
# Rate limiting for alarms (prevents spam)
alarm_rate = 300              # Minimum seconds between alarms per host

# Default alarm receiver script
alarm_receiver = "alarmreceiver.sh"
```

## 🎯 **Alarm Triggers**

Alarms are triggered when **any** of these thresholds are exceeded:

### **1. Ping Time (alarmping)**
- **Unit**: Milliseconds
- **Trigger**: Average RTT > threshold
- **Example**: `alarmping = 250` triggers when RTT > 250ms

### **2. Packet Loss (alarmloss)**
- **Unit**: Percentage
- **Trigger**: Packet loss > threshold  
- **Example**: `alarmloss = 5` triggers when loss > 5%

### **3. Jitter (alarmjitter)**
- **Unit**: Milliseconds
- **Trigger**: Jitter (RTT standard deviation) > threshold
- **Example**: `alarmjitter = 100` triggers when jitter > 100ms

## 📞 **Alarm Receiver Scripts**

### **Script Selection Priority**
1. **Host-specific**: `alarmreceiver` field in host config
2. **Global default**: `alarm_receiver` in main config
3. **Built-in**: Log-only fallback

### **Performance Optimization: Alarm Filtering**
SmogPing includes intelligent alarm filtering to improve performance:

- **No alarmreceiver defined**: Alarm checks are skipped entirely for this host
- **alarmreceiver = "none"**: Alarm checks are skipped (case-insensitive)
- **alarmreceiver = script**: Normal alarm processing

```toml
[organizations.example]
hosts = [
  { name = "critical", ip = "10.0.1.10", alarmreceiver = "./urgent-alarm.sh" },  # Full alarm processing
  { name = "monitored", ip = "10.0.2.10" },                                      # No alarm receiver = no alarm checks
  { name = "stats-only", ip = "10.0.3.10", alarmreceiver = "none" },            # Explicitly disabled alarms
]
```

This optimization reduces CPU usage for hosts that only need monitoring without alerting.

### **Script Execution**
Scripts are called with both **command-line arguments** and **environment variables**:

#### **Command Line Arguments**
```bash
./alarmreceiver.sh "$HOST_NAME" "$HOST_IP" "$ORG" "$RTT_MS" "$LOSS_%" "$JITTER_MS" "$REASONS" "$TIMESTAMP"
```

#### **Environment Variables**
```bash
SMOGPING_HOST="server-name"
SMOGPING_IP="192.168.1.10"
SMOGPING_ORG="production"
SMOGPING_RTT="275.3"           # RTT in milliseconds
SMOGPING_LOSS="7.2"            # Packet loss percentage
SMOGPING_JITTER="120.5"        # Jitter in milliseconds
SMOGPING_REASONS="ping_time=275.3ms>250ms,packet_loss=7.2%>5%"
SMOGPING_TIMESTAMP="2025-07-28T10:30:00Z"
SMOGPING_ALARM_PING="250"      # Configured thresholds
SMOGPING_ALARM_LOSS="5"
SMOGPING_ALARM_JITTER="100"
```

## 🛡️ **Alarm Rate Limiting**

### **Purpose**
Prevents alarm flooding when a host has persistent issues.

### **Behavior**
- **First alarm**: Executes immediately
- **Subsequent alarms**: Suppressed for `alarm_rate` seconds
- **Per-host tracking**: Each host has independent rate limiting

### **Example**
With `alarm_rate = 300` (5 minutes):
```
10:00:00 - Alarm triggered → Script executed
10:02:00 - Alarm triggered → Suppressed (within 5 min)
10:05:01 - Alarm triggered → Script executed (5+ min elapsed)
```

## 📋 **Example Alarm Scenarios**

### **High Latency Alarm**
```
Host: "Database Server" (10.0.1.50)
Threshold: alarmping = 200
Result: RTT = 350ms
Trigger: ping_time=350.0ms>200ms
```

### **Packet Loss Alarm**
```
Host: "Web Frontend" (10.0.2.100) 
Threshold: alarmloss = 3
Result: Loss = 8.5%
Trigger: packet_loss=8.5%>3%
```

### **Multiple Threshold Alarm**
```
Host: "API Gateway" (10.0.3.200)
Thresholds: alarmping=150, alarmloss=2, alarmjitter=50
Result: RTT=200ms, Loss=5%, Jitter=75ms
Trigger: ping_time=200.0ms>150ms,packet_loss=5.0%>2%,jitter=75.0ms>50ms
```

## 🔧 **Sample Alarm Receiver Script**

The included `alarmreceiver.sh` demonstrates common alarm actions:

```bash
#!/bin/bash
HOST_NAME="$1"
HOST_IP="$2"
ALARM_REASONS="$7"

# Log to syslog
logger -t smogping "ALARM: $HOST_NAME ($HOST_IP) - $ALARM_REASONS"

# Write to alarm log
echo "$(date -Iseconds) ALARM $HOST_NAME $HOST_IP $ALARM_REASONS" >> /var/log/smogping-alarms.log

# Send email (customize as needed)
# echo "Network alarm: $HOST_NAME - $ALARM_REASONS" | mail -s "SmogPing Alert" admin@example.com

# HTTP POST to monitoring system
# curl -X POST "http://monitoring.example.com/alerts" \
#   -H "Content-Type: application/json" \
#   -d "{\"host\":\"$HOST_NAME\", \"reasons\":\"$ALARM_REASONS\"}"
```

## 🚀 **Integration Examples**

### **1. Email Notifications**
```bash
# Requires mail command
echo "SmogPing Alert: $HOST_NAME ($HOST_IP) - $ALARM_REASONS" | \
  mail -s "Network Alarm" -r "smogping@company.com" admin@company.com
```

### **2. Slack Integration**
```bash
SLACK_WEBHOOK="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
curl -X POST -H 'Content-type: application/json' \
  --data "{\"text\":\"🚨 Network Alert: $HOST_NAME ($HOST_IP) - $ALARM_REASONS\"}" \
  "$SLACK_WEBHOOK"
```

### **3. PagerDuty Integration**
```bash
PAGERDUTY_KEY="your-integration-key"
curl -X POST "https://events.pagerduty.com/v2/enqueue" \
  -H "Content-Type: application/json" \
  -d "{
    \"routing_key\": \"$PAGERDUTY_KEY\",
    \"event_action\": \"trigger\",
    \"payload\": {
      \"summary\": \"SmogPing Alert: $HOST_NAME - $ALARM_REASONS\",
      \"source\": \"smogping\",
      \"severity\": \"error\"
    }
  }"
```

### **4. SNMP Traps**
```bash
# Requires net-snmp-utils
snmptrap -v2c -c public localhost '' 1.3.6.1.4.1.12345.1 \
  1.3.6.1.4.1.12345.1.1 s "$HOST_NAME" \
  1.3.6.1.4.1.12345.1.2 s "$HOST_IP" \
  1.3.6.1.4.1.12345.1.3 s "$ALARM_REASONS"
```

### **5. Custom Monitoring Systems**
```bash
# HTTP POST with detailed metrics
curl -X POST "http://your-monitoring-system.com/api/alerts" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-token" \
  -d "{
    \"timestamp\": \"$SMOGPING_TIMESTAMP\",
    \"host\": \"$SMOGPING_HOST\",
    \"ip\": \"$SMOGPING_IP\",
    \"organization\": \"$SMOGPING_ORG\",
    \"metrics\": {
      \"rtt_ms\": $SMOGPING_RTT,
      \"loss_percent\": $SMOGPING_LOSS,
      \"jitter_ms\": $SMOGPING_JITTER
    },
    \"thresholds\": {
      \"rtt_ms\": $SMOGPING_ALARM_PING,
      \"loss_percent\": $SMOGPING_ALARM_LOSS,
      \"jitter_ms\": $SMOGPING_ALARM_JITTER
    },
    \"alarm_reasons\": \"$SMOGPING_REASONS\"
  }"
```

## 🔍 **Troubleshooting**

### **Alarms Not Triggering**
1. Check threshold configuration in target files
2. Verify alarm receiver script path and permissions
3. Check alarm rate limiting (may be suppressed)
4. Review SmogPing logs for alarm evaluation

### **Script Execution Failures**
1. Verify script permissions (`chmod +x`)
2. Check script syntax and dependencies
3. Test script manually with sample data
4. Review SmogPing logs for error messages

### **Alarm Flooding**
1. Increase `alarm_rate` value (longer suppression)
2. Adjust alarm thresholds (less sensitive)
3. Fix underlying network issues
4. Implement exponential backoff in alarm scripts

## 📊 **Alarm Monitoring**

### **Log Output Examples**
```
ALARM: Database Server (10.0.1.50) - [ping_time=350.0ms>200ms] - Executing: ./alarmreceiver.sh
Alarm receiver completed for Database Server (10.0.1.50) - Output: Alert sent successfully
```

### **Performance Impact**
- Alarm checking adds minimal overhead (~1ms per result)
- Scripts execute asynchronously (non-blocking)
- Rate limiting prevents excessive script execution
- Failed scripts timeout after 30 seconds
