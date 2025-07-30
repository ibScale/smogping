# SmogPing InfluxDB Batching

## 📦 **Batching Implementation**

SmogPing implements intelligent batching for InfluxDB writes to optimize performance and prevent data loss.

## ⚙️ **Batching Configuration**

```toml
# InfluxDB batch size for writes
influx_batch_size = 100

# InfluxDB batch time in seconds - flush if batch_size not reached
influx_batch_time = 10
```

## 🔄 **How Batching Works**

### **Dual Flush Triggers**
1. **Size-based flushing**: When batch reaches `influx_batch_size` points
2. **Time-based flushing**: Every `influx_batch_time` seconds (if batch has data)

### **Example with 869 Targets**
- **Data points per cycle**: 869 (one per target every 60 seconds)
- **With batch_size=100**: Will flush ~9 times per cycle
- **Timing**: First flush at 100 points, subsequent flushes as batches fill
- **Safety**: Timer ensures no data hangs longer than 10 seconds

## 📊 **Batching Behavior Examples**

### **High-Volume Scenario** (1000+ targets):
```
Batch 1: Collects 100 points → Flushes (reason: size)
Batch 2: Collects 100 points → Flushes (reason: size)  
...continuing until all targets complete
```

### **Low-Volume Scenario** (50 targets):
```
Batch 1: Collects 50 points over ~60 seconds → Flushes (reason: timer)
```

### **Mixed Scenario** (150 targets):
```
Batch 1: Collects 100 points → Flushes (reason: size)
Batch 2: Collects 50 points → Waits for timer → Flushes (reason: timer)
```

## 🚨 **Data Safety Features**

### **Graceful Shutdown**
- Final flush on application shutdown ensures no data loss
- All pending points written before exit

### **Timer-Based Safety Net**
- Background timer prevents data from hanging indefinitely
- Configurable timeout ensures timely writes

### **Thread-Safe Operations**
- Mutex protection for concurrent access
- Safe for multiple goroutines writing simultaneously

## 📈 **Performance Benefits**

### **Reduced InfluxDB Load**
- **Before**: 869 individual writes per cycle
- **After**: ~9 batched writes per cycle (with batch_size=100)
- **Improvement**: ~97% reduction in write operations

### **Better Network Efficiency**
- Fewer HTTP requests to InfluxDB
- Reduced connection overhead
- More efficient TCP utilization

### **Improved Throughput**
- Higher data ingestion rates
- Better InfluxDB write performance
- Reduced write latency overall

## 🔧 **Tuning Guidelines**

### **For High-Volume Deployments** (500+ targets):
```toml
influx_batch_size = 500      # Larger batches for efficiency
influx_batch_time = 30       # Longer time window acceptable
```

### **For Real-Time Requirements** (low latency needs):
```toml
influx_batch_size = 50       # Smaller batches for faster writes
influx_batch_time = 5        # Frequent flushes for low latency
```

### **For Resource-Constrained Systems**:
```toml
influx_batch_size = 25       # Smaller memory footprint
influx_batch_time = 15       # Less frequent writes
```

## 📋 **Monitoring Batch Performance**

### **Log Output Examples**
```
InfluxDB batching configured: BatchSize=100, BatchTime=10s
Flushed 100 points to InfluxDB (reason: size)
Flushed 69 points to InfluxDB (reason: timer)
Flushed 150 points to InfluxDB (reason: shutdown)
```

### **What to Watch For**
- **Frequent size flushes**: System working efficiently
- **Timer-only flushes**: Low volume, consider reducing batch size
- **Large shutdown flushes**: Consider shorter batch time

## ⚠️ **Important Considerations**

### **Memory Usage**
- Each batch holds points in memory until flushed
- Memory usage = `batch_size × point_size × active_batches`
- Monitor memory consumption with large batch sizes

### **Data Freshness vs. Efficiency**
- **Larger batches**: More efficient, less real-time
- **Smaller batches**: More real-time, less efficient
- **Balance based on requirements**

### **InfluxDB Configuration**
- Ensure InfluxDB can handle your batch sizes
- Monitor InfluxDB memory and write performance
- Consider InfluxDB's own batching settings

## 🔍 **Troubleshooting**

### **Data Not Appearing Promptly**
- Check `influx_batch_time` setting
- Verify InfluxDB connectivity
- Monitor flush log messages

### **High Memory Usage**
- Reduce `influx_batch_size`
- Check for InfluxDB connectivity issues causing backup
- Monitor batch accumulation

### **Poor Write Performance**
- Increase `influx_batch_size` for fewer writes
- Check InfluxDB server performance
- Monitor network latency to InfluxDB
