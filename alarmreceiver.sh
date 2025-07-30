#!/bin/bash
# SmogPing Alarm Receiver Script
# This script is called when alarm thresholds are exceeded

# Command line arguments
HOST_NAME="$1"
HOST_IP="$2"
ORGANIZATION="$3"
RTT_MS="$4"
PACKET_LOSS="$5"
JITTER_MS="$6"
ALARM_REASONS="$7"
TIMESTAMP="$8"

# Environment variables are also available:
# SMOGPING_HOST, SMOGPING_IP, SMOGPING_ORG, SMOGPING_RTT, 
# SMOGPING_LOSS, SMOGPING_JITTER, SMOGPING_REASONS, SMOGPING_TIMESTAMP
# SMOGPING_ALARM_PING, SMOGPING_ALARM_LOSS, SMOGPING_ALARM_JITTER

# Log the alarm
echo "$(date): ALARM for $HOST_NAME ($HOST_IP) in $ORGANIZATION"
echo "  RTT: ${RTT_MS}ms, Loss: ${PACKET_LOSS}%, Jitter: ${JITTER_MS}ms"
echo "  Reasons: $ALARM_REASONS"
echo "  Timestamp: $TIMESTAMP"

# Example actions you can implement:

# 1. Log to syslog
logger -t smogping "ALARM: $HOST_NAME ($HOST_IP) - $ALARM_REASONS"

# 2. Send email (requires mail command)
# echo "Network alarm for $HOST_NAME ($HOST_IP): $ALARM_REASONS" | mail -s "SmogPing Alert" admin@example.com

# 3. Send to Slack (requires curl and webhook URL)
# SLACK_WEBHOOK="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
# curl -X POST -H 'Content-type: application/json' \
#   --data "{\"text\":\"🚨 SmogPing Alert: $HOST_NAME ($HOST_IP) - $ALARM_REASONS\"}" \
#   "$SLACK_WEBHOOK"

# 4. Write to alarm log file
ALARM_LOG="/var/log/smogping-alarms.log"
echo "$(date -Iseconds) ALARM $HOST_NAME $HOST_IP $ORGANIZATION RTT=${RTT_MS}ms LOSS=${PACKET_LOSS}% JITTER=${JITTER_MS}ms REASONS=\"$ALARM_REASONS\"" >> "$ALARM_LOG"

# 5. Send SNMP trap (requires snmptrap command)
# snmptrap -v2c -c public localhost '' 1.3.6.1.4.1.12345.1 \
#   1.3.6.1.4.1.12345.1.1 s "$HOST_NAME" \
#   1.3.6.1.4.1.12345.1.2 s "$HOST_IP" \
#   1.3.6.1.4.1.12345.1.3 s "$ALARM_REASONS"

# 6. HTTP POST to monitoring system (requires curl)
# curl -X POST "http://monitoring.example.com/alerts" \
#   -H "Content-Type: application/json" \
#   -d "{\"host\":\"$HOST_NAME\", \"ip\":\"$HOST_IP\", \"reasons\":\"$ALARM_REASONS\", \"rtt\":$RTT_MS, \"loss\":$PACKET_LOSS, \"jitter\":$JITTER_MS}"

# Exit successfully
exit 0
