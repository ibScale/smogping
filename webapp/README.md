# SmogPing Web Dashboard

A professional web interface for monitoring SmogPing network performance data stored in InfluxDB.

## 📁 Directory Structure

```
webapp/
├── README.md              # This file
├── dashboard.php          # Main dashboard application
├── dashboard_template.php # HTML template for dashboard
├── charts.php            # Advanced charts interface
├── config.example.php     # Configuration template
└── DASHBOARD_README.md    # Detailed documentation
```

## 🚀 Quick Start

### 1. Copy Configuration
```bash
cd webapp/
cp config.example.php config.php
```

### 2. Edit Configuration
```bash
nano config.php
```

Update with your InfluxDB settings:
```php
<?php
return [
    'influxdb' => [
        'url' => 'http://localhost:8086',
        'token' => 'your-influxdb-token-here',
        'org' => 'your-organization-name',
        'bucket' => 'your-bucket-name'
    ]
];
?>
```

### 3. Start Web Server

#### Option A: PHP Built-in Server (Development)
```bash
cd webapp/
php -S localhost:8080
```

#### Option B: Apache/Nginx (Production)
```bash
# Copy to web server directory
sudo cp -r webapp/ /var/www/html/smogping/

# Set permissions
sudo chown -R www-data:www-data /var/www/html/smogping/
sudo chmod -R 755 /var/www/html/smogping/
```

### 4. Access Dashboard
- **Main Dashboard**: http://localhost:8080/dashboard.php
- **Charts View**: http://localhost:8080/charts.php

## 📊 Features

### Main Dashboard (`dashboard.php`)
- Real-time network performance metrics
- Organization and host filtering
- Time range selection (5 minutes to 7 days)
- Auto-refresh functionality
- Recent results table with status indicators
- DNS resolution tracking
- Mobile-responsive design

### Charts Interface (`charts.php`)
- Interactive time-series charts
- RTT, packet loss, and jitter trends
- Multi-host comparison
- Zoom and pan capabilities
- Combined performance overview

## 🔧 Requirements

- **PHP 7.4+** with curl extension
- **Web server** (Apache, Nginx, or PHP built-in)
- **InfluxDB v2** with SmogPing data
- **Modern web browser** with JavaScript enabled

## 📱 Browser Support

- Chrome/Chromium 60+
- Firefox 55+
- Safari 12+
- Edge 79+
- Mobile browsers (iOS Safari, Chrome Mobile)

## 🛡️ Security Notes

### Development
- Use PHP built-in server for testing only
- Keep configuration files secure

### Production
- Use HTTPS with proper SSL certificates
- Protect config.php from web access
- Consider environment variables for sensitive data
- Implement access controls if needed

## 📈 Performance Tips

### For Large Datasets
- Use shorter time ranges for better performance
- Consider InfluxDB continuous queries for aggregation
- Implement caching for frequently accessed data

### Network Optimization
- Enable gzip compression
- Use CDN for Chart.js assets in production
- Optimize InfluxDB queries

## 🔗 Integration

The webapp integrates seamlessly with SmogPing data:

### Expected Data Format
```
Measurement: "ping"
Tags: host, ip, organization, source, resolved_ip, is_dns_name
Fields: rtt_avg (ms), packet_loss (%), jitter (ms)
```

### SmogPing Configuration
Ensure your SmogPing application is configured to write to the same InfluxDB instance specified in `config.php`.

## 📚 Documentation

For detailed setup, customization, and troubleshooting information, see:
- **[DASHBOARD_README.md](DASHBOARD_README.md)** - Complete documentation

## 🆘 Support

### Common Issues
1. **No data displayed**: Check InfluxDB connection and SmogPing data
2. **Permission errors**: Verify file permissions and web server configuration
3. **Chart loading issues**: Ensure internet connection for Chart.js CDN

### Debugging
```bash
# Enable PHP error reporting
echo "<?php error_reporting(E_ALL); ini_set('display_errors', 1); ?>" > debug.php

# Check PHP logs
tail -f /var/log/php_errors.log

# Test InfluxDB connection
curl -H "Authorization: Token YOUR_TOKEN" "http://localhost:8086/api/v2/ping"
```

## 🎯 Next Steps

1. Configure your InfluxDB connection
2. Start the web server
3. Access the dashboard
4. Customize the interface for your needs
5. Set up production deployment if needed

The SmogPing Web Dashboard provides professional-grade network monitoring visualization with enterprise features suitable for both development and production environments.
