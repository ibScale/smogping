# SmogPing PHP Dashboard

A comprehensive web dashboard for viewing SmogPing network monitoring results from InfluxDB.

## 📋 Features

- **Real-time Dashboard** - View current network performance metrics
- **Interactive Charts** - Time-series graphs with Chart.js
- **Organization & Host Filtering** - Drill down into specific targets
- **Auto-refresh** - Automatic data updates
- **Responsive Design** - Works on desktop and mobile
- **DNS Resolution Tracking** - Shows both DNS names and resolved IPs
- **Performance Statistics** - RTT, packet loss, and jitter metrics

## 🚀 Quick Setup

### 1. Prerequisites
```bash
# PHP 7.4+ with curl extension
sudo apt update
sudo apt install php php-curl php-cli

# Web server (Apache/Nginx) or use PHP built-in server
```

### 2. Installation
```bash
# Copy dashboard files to web directory
cp dashboard.php /var/www/html/smogping/
cp dashboard_template.php /var/www/html/smogping/
cp charts.php /var/www/html/smogping/
cp config.example.php /var/www/html/smogping/

# Create configuration
cd /var/www/html/smogping/
cp config.example.php config.php
```

### 3. Configuration
Edit `config.php` with your InfluxDB settings:

```php
<?php
return [
    'influxdb' => [
        'url' => 'http://localhost:8086',
        'token' => 'your-influxdb-token-here',
        'org' => 'your-organization-name',
        'bucket' => 'your-bucket-name'
    ],
    'dashboard' => [
        'title' => 'My Network Monitor',
        'refresh_interval' => 30,
        'default_timerange' => '-1h',
        'items_per_page' => 100
    ]
];
?>
```

### 4. Start Web Server
```bash
# Option 1: PHP built-in server (development)
cd /var/www/html/smogping/
php -S localhost:8080

# Option 2: Apache/Nginx (production)
# Configure virtual host to point to the dashboard directory
```

### 5. Access Dashboard
- Main Dashboard: `http://localhost:8080/dashboard.php`
- Charts View: `http://localhost:8080/charts.php`

## 📊 Dashboard Features

### Main Dashboard (`dashboard.php`)
- **Overall Statistics** - Average RTT, packet loss, and jitter
- **Organization Summary** - List of monitored organizations
- **Time Range Selection** - 5 minutes to 7 days
- **Filtering Options** - By organization and host
- **Recent Results Table** - Latest ping results with status indicators
- **Auto-refresh** - Optional 30-second automatic updates
- **DNS Information** - Shows original hostnames and resolved IPs

### Charts View (`charts.php`)
- **RTT Time Series** - Round-trip time trends
- **Packet Loss Graphs** - Loss percentage over time
- **Jitter Analysis** - Network stability metrics
- **Combined Overview** - Multi-metric comparison
- **Interactive Legends** - Toggle data series
- **Zoom & Pan** - Detailed time range analysis

## 🎨 Data Structure

The dashboard expects InfluxDB data in this format:
```
Measurement: "ping"

Tags:
- host: Host name (e.g., "Google DNS Primary")
- ip: Original IP/hostname (e.g., "8.8.8.8" or "google.com")
- organization: Organization name (e.g., "PublicDNS")
- source: Ping source (e.g., "monitoring-server")
- resolved_ip: Resolved IP (for DNS names)
- is_dns_name: "true" or "false"

Fields:
- rtt_avg: Average round-trip time (milliseconds)
- packet_loss: Packet loss percentage (0-100)
- jitter: Jitter/variation (milliseconds)
```

## 📱 Mobile Support

The dashboard is fully responsive and includes:
- **Mobile-optimized layout** - Stacked cards on small screens
- **Touch-friendly controls** - Large buttons and select boxes
- **Readable tables** - Horizontal scrolling for data tables
- **Optimized charts** - Responsive Chart.js configurations

## 🔧 Customization

### Themes & Styling
Edit the CSS in `dashboard_template.php` to customize:
- Color schemes
- Layout arrangements
- Typography
- Card styling

### Time Ranges
Add custom time ranges in the dropdown:
```php
<option value="-2h">Last 2 Hours</option>
<option value="-12h">Last 12 Hours</option>
<option value="-14d">Last 2 Weeks</option>
```

### Alert Thresholds
Modify status thresholds in the results table:
```php
// In dashboard_template.php
if ($rtt > 500 || $loss > 10 || $jitter > 200) {
    $status = 'critical';
} elseif ($rtt > 200 || $loss > 5 || $jitter > 100) {
    $status = 'warning';
}
```

## 🛠️ Troubleshooting

### Common Issues

#### 1. InfluxDB Connection Errors
```bash
# Check InfluxDB status
curl -I http://localhost:8086/ping

# Verify token permissions
influx auth list

# Test connection
curl -H "Authorization: Token YOUR_TOKEN" \
  "http://localhost:8086/api/v2/query?org=YOUR_ORG" \
  -d 'from(bucket:"YOUR_BUCKET")|>range(start:-1h)|>limit(n:1)'
```

#### 2. No Data Displayed
- Verify SmogPing is running and writing to InfluxDB
- Check bucket name matches configuration
- Ensure time range includes data points
- Verify organization/host names match data

#### 3. PHP Errors
```bash
# Enable error reporting
echo "error_reporting(E_ALL); ini_set('display_errors', 1);" > debug.php

# Check PHP logs
tail -f /var/log/php_errors.log

# Verify curl extension
php -m | grep curl
```

#### 4. Permission Issues
```bash
# Set proper permissions
sudo chown -R www-data:www-data /var/www/html/smogping/
sudo chmod -R 755 /var/www/html/smogping/
```

## 🔐 Security Considerations

### Production Deployment
1. **Hide configuration files**
   ```apache
   # .htaccess
   <Files "config.php">
       Order deny,allow
       Deny from all
   </Files>
   ```

2. **Use environment variables**
   ```php
   'token' => $_ENV['INFLUXDB_TOKEN'] ?? 'default-token',
   ```

3. **Enable HTTPS**
   ```bash
   # Install SSL certificate
   sudo certbot --apache -d your-domain.com
   ```

4. **Input validation**
   - Time ranges are validated
   - Organization/host names are sanitized
   - SQL injection protection via parameterized queries

## 📈 Performance Tips

### Large Datasets
- Use shorter time ranges for better performance
- Implement pagination for large result sets
- Consider data aggregation for long-term views
- Use InfluxDB continuous queries for pre-aggregated data

### Caching
```php
// Add simple caching
$cacheFile = "cache_" . md5($flux) . ".json";
if (file_exists($cacheFile) && (time() - filemtime($cacheFile)) < 60) {
    return json_decode(file_get_contents($cacheFile), true);
}
// ... query and cache result
```

## 🆘 Support

For issues with:
- **SmogPing application**: Check main.go and application logs
- **InfluxDB**: Verify connection, permissions, and data format
- **Dashboard**: Check PHP logs and browser console
- **Charts**: Verify Chart.js loading and data format

The dashboard provides comprehensive network monitoring visualization with professional-grade features suitable for both development and production environments.
