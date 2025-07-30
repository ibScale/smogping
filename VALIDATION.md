# SmogPing Configuration Validation

## 🔍 **Overview**

SmogPing performs comprehensive configuration validation on startup, including TOML syntax checking, value validation, and performance feasibility analysis.

## 🛡️ **Validation Types**

### **1. File & TOML Validation**
- ✅ **File existence and permissions**
- ✅ **TOML syntax validation** with line/column error reporting
- ✅ **Field validation** (required fields, ranges, formats)
- ✅ **Unknown field warnings**

### **2. Performance & Capacity Validation**
- ✅ **Target count vs capacity checks**
- ✅ **Ping timing feasibility**
- ✅ **Rate limiting validation**
- ✅ **Resource utilization warnings**

## ⚠️ **Critical Validations**

### **Target Count vs Capacity**
**Formula**: `Max Targets = max_concurrent_pings × data_point_time`

**Examples**:
- `50 concurrent × 60 seconds = 3,000 max targets`
- `100 concurrent × 60 seconds = 6,000 max targets`

**Error Conditions**:
- **Fatal Error**: Target count exceeds theoretical maximum
- **Warning**: Target count ≥80% of maximum capacity

### **Ping Timing Validation**
**Checks**:
- Ping interval = `data_point_time ÷ data_point_pings`
- Warns if interval < 1 second (too aggressive)
- Warns if `ping_timeout > ping_interval` (overlapping pings)

### **Rate Limiting & InfluxDB**
**Validation**: Can all pings complete within the data point time?
- Total pings needed = `total_hosts × data_point_pings`
- Time needed = `total_pings ÷ ping_rate_limit`
- Warns if time needed > `data_point_time`
- Checks `influx_batch_size > 0` and `influx_batch_time > 0`

## 🚨 **Common Error Types**

### **TOML Parse Errors**
```
TOML parse error in config.toml at line 5, column 12: expected key separator '=' but found ':'

Context:
  4: influx_url = "http://localhost:8086"
> 5: influx_token : "your_token_here"
  6: influx_org = "my_org"
```

### **Validation Errors**
```
TOML validation error in config.toml: influx_batch_size = 20000 - must be between 0 and 10000
```

### **Capacity Errors**
```
Configuration validation failed: Target count (3500) exceeds theoretical maximum (3000).
Consider increasing max_concurrent_pings from 50 to 60 or data_point_time from 60 to 70 seconds.
```

## 📋 **Configuration Field Validation**

### **Required Fields & Ranges**
| Field | Type | Range/Format | Notes |
|-------|------|--------------|-------|
| `influx_url` | URL | http:// or https:// | Must be valid URL |
| `influx_token` | String | Any | Required for auth |
| `influx_batch_size` | Integer | 0-10000 | Performance setting |
| `data_point_pings` | Integer | 1-100 | Pings per measurement |
| `data_point_time` | Integer | 1-86400 | Interval (seconds) |
| `ping_timeout` | Integer | 1-60 | Individual ping timeout |
| `max_concurrent_pings` | Integer | 1-1000 | Concurrency limit |

## � **Error Resolution**

### **"Target count exceeds theoretical maximum"**
**Solutions**:
1. Increase concurrent capacity: `max_concurrent_pings = 100`
2. Increase time window: `data_point_time = 120`
3. Reduce targets or split deployment

### **"Rate limiting prevents completion"**
**Solutions**:
1. Increase rate limit: `ping_rate_limit = 50`
2. Reduce pings per point: `data_point_pings = 3`
3. Increase time window: `data_point_time = 120`

### **TOML Syntax Issues**
- **Missing `=`**: Use `key = value` not `key : value`
- **Unmatched quotes**: Ensure all strings are properly quoted
- **Invalid characters**: Check for special characters in values

## 📈 **Capacity Planning**

**Formulas**:
```
Max Targets = max_concurrent_pings × data_point_time
Ping Interval = data_point_time ÷ data_point_pings
Total Pings/Cycle = target_count × data_point_pings
```

**Example Planning**:
- Want to monitor 5,000 targets with 60-second intervals?
- Need `5000 ÷ 60 = 84` concurrent pings minimum
- Set `max_concurrent_pings = 100` for safety margin
