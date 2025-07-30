<?php
// SPDX-License-Identifier: GPL-3.0
// Copyright (C) 2025 FexTel, Inc. <info@ibscale.com>
// Author: James Pearson <jamesp@ibscale.com>

/**
 * SmogPing Web Dashboard
 * PHP application for viewing SmogPing network monitoring results from InfluxDB
 */

// Configuration
$config = [
    'influxdb' => [
        'url' => 'http://localhost:8086',
        'token' => 'YOUR_TOKEN',
        'org' => 'YOUR_ORG',
        'bucket' => 'YOUR_BUCKET'
    ],
    'dashboard' => [
        'title' => 'SmogPing Network Monitor',
        'refresh_interval' => 30, // seconds
        'default_timerange' => '-1h', // Last hour
        'items_per_page' => 50
    ]
];

// Load configuration from file if exists
$configPaths = [
    '/etc/smogping/config.php',  // System installation
    __DIR__ . '/config.php'             // Development/local
];

foreach ($configPaths as $configPath) {
    if (file_exists($configPath)) {
        define('SMOGPING_DASHBOARD', true);
        $userConfig = include $configPath;
        $config = array_replace_recursive($config, $userConfig);
        break;
    }
}

class InfluxDBClient {
    private $url;
    private $token;
    private $org;
    private $bucket;
    
    public function __construct($config) {
        // Validate configuration
        if (!is_array($config) || !isset($config['url'], $config['token'], $config['org'], $config['bucket'])) {
            throw new Exception('Invalid InfluxDB configuration. Required: url, token, org, bucket');
        }
        
        if (!is_string($config['url'])) {
            throw new Exception('InfluxDB URL must be a string');
        }
        
        $this->url = rtrim($config['url'], '/');
        $this->token = $config['token'];
        $this->org = $config['org'];
        $this->bucket = $config['bucket'];
    }
    
    public function query($flux) {
        $ch = curl_init();
        
        // Build the query URL with org parameter
        $queryUrl = $this->url . '/api/v2/query?org=' . urlencode($this->org);
        
        curl_setopt_array($ch, [
            CURLOPT_URL => $queryUrl,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_POST => true,
            CURLOPT_POSTFIELDS => $flux,
            CURLOPT_HTTPHEADER => [
                'Authorization: Token ' . $this->token,
                'Accept: application/csv',
                'Content-Type: application/vnd.flux'
            ]
        ]);
        
        $response = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        
        if (curl_error($ch)) {
            throw new Exception('Curl error: ' . curl_error($ch));
        }
        
        curl_close($ch);
        
        if ($httpCode !== 200) {
            throw new Exception('InfluxDB error: HTTP ' . $httpCode . ' - ' . $response);
        }
        
        return $this->parseCSV($response);
    }
    
    private function parseCSV($csv) {
        $lines = explode("\n", trim($csv));
        $results = [];
        $headers = null;
        
        foreach ($lines as $line) {
            if (empty($line) || strpos($line, '#') === 0) {
                continue;
            }
            
            $data = str_getcsv($line);
            
            if ($headers === null) {
                $headers = $data;
                continue;
            }
            
            if (count($data) === count($headers)) {
                $results[] = array_combine($headers, $data);
            }
        }
        
        return $results;
    }
}

class SmogPingDashboard {
    private $influx;
    private $config;
    
    public function __construct($config) {
        $this->config = $config;
        $this->influx = new InfluxDBClient($config['influxdb']);
    }
    
    public function getRecentResults($timeRange = '-1h', $limit = 100) {
        // Use simpler query without pivot
        $flux = '
            from(bucket: "' . $this->config['influxdb']['bucket'] . '")
            |> range(start: ' . $timeRange . ')
            |> filter(fn: (r) => r["_measurement"] == "ping")
            |> filter(fn: (r) => r["_field"] == "rtt_avg" or r["_field"] == "packet_loss" or r["_field"] == "jitter")
            |> sort(columns: ["_time"], desc: true)
            |> limit(n: ' . ($limit * 3) . ')
        ';
        
        $rawResults = $this->influx->query($flux);
        
        // Group by time and restructure data
        $timeGroups = [];
        foreach ($rawResults as $row) {
            $time = $row['_time'];
            if (!isset($timeGroups[$time])) {
                $timeGroups[$time] = [
                    '_time' => $time,
                    'host' => $row['host'] ?? '',
                    'organization' => $row['organization'] ?? '',
                    'ip' => $row['ip'] ?? ''
                ];
            }
            $timeGroups[$time][$row['_field']] = floatval($row['_value'] ?? 0);
        }
        
        // Sort by time descending and limit results
        $results = array_values($timeGroups);
        usort($results, function($a, $b) {
            return strtotime($b['_time']) - strtotime($a['_time']);
        });
        
        return array_slice($results, 0, $limit);
    }
    
    public function getOrganizations($timeRange = '-1h') {
        $flux = '
            from(bucket: "' . $this->config['influxdb']['bucket'] . '")
            |> range(start: ' . $timeRange . ')
            |> filter(fn: (r) => r["_measurement"] == "ping")
            |> filter(fn: (r) => r["_field"] == "rtt_avg")
            |> keep(columns: ["organization"])
            |> distinct(column: "organization")
            |> sort(columns: ["organization"])
        ';
        
        $results = $this->influx->query($flux);
        return array_unique(array_column($results, 'organization'));
    }
    
    public function getHostsByOrganization($org, $timeRange = '-1h') {
        $flux = '
            from(bucket: "' . $this->config['influxdb']['bucket'] . '")
            |> range(start: ' . $timeRange . ')
            |> filter(fn: (r) => r["_measurement"] == "ping")
            |> filter(fn: (r) => r["organization"] == "' . $org . '")
            |> filter(fn: (r) => r["_field"] == "rtt_avg")
            |> keep(columns: ["host", "ip"])
            |> distinct(column: "host")
            |> sort(columns: ["host"])
        ';
        
        return $this->influx->query($flux);
    }
    
    public function getHostStats($host, $org, $timeRange = '-1h') {
        // Use simpler query without pivot
        $flux = '
            from(bucket: "' . $this->config['influxdb']['bucket'] . '")
            |> range(start: ' . $timeRange . ')
            |> filter(fn: (r) => r["_measurement"] == "ping")
            |> filter(fn: (r) => r["host"] == "' . $host . '")
            |> filter(fn: (r) => r["organization"] == "' . $org . '")
            |> filter(fn: (r) => r["_field"] == "rtt_avg" or r["_field"] == "packet_loss" or r["_field"] == "jitter")
            |> sort(columns: ["_time"], desc: false)
        ';
        
        $rawResults = $this->influx->query($flux);
        
        // Group by time and restructure data
        $timeGroups = [];
        foreach ($rawResults as $row) {
            $time = $row['_time'];
            if (!isset($timeGroups[$time])) {
                $timeGroups[$time] = [
                    '_time' => $time,
                    'host' => $row['host'] ?? '',
                    'organization' => $row['organization'] ?? ''
                ];
            }
            $timeGroups[$time][$row['_field']] = floatval($row['_value'] ?? 0);
        }
        
        return array_values($timeGroups);
    }
    
    public function getHostTimeSeriesStats($host, $org, $timeRange = '-1h') {
        // Use a simpler query without pivot that's more reliable
        $flux = '
            from(bucket: "' . $this->config['influxdb']['bucket'] . '")
            |> range(start: ' . $timeRange . ')
            |> filter(fn: (r) => r["_measurement"] == "ping")
            |> filter(fn: (r) => r["host"] == "' . $host . '")
            |> filter(fn: (r) => r["organization"] == "' . $org . '")
            |> filter(fn: (r) => r["_field"] == "rtt_avg" or r["_field"] == "packet_loss" or r["_field"] == "jitter")
            |> sort(columns: ["_time"], desc: false)
        ';
        
        $rawResults = $this->influx->query($flux);
        
        // Group by time and restructure data
        $timeGroups = [];
        foreach ($rawResults as $row) {
            $time = $row['_time'];
            if (!isset($timeGroups[$time])) {
                $timeGroups[$time] = [
                    '_time' => $time,
                    'host' => $row['host'] ?? '',
                    'organization' => $row['organization'] ?? ''
                ];
            }
            $timeGroups[$time][$row['_field']] = floatval($row['_value'] ?? 0);
        }
        
        return array_values($timeGroups);
    }

    public function getAggregateStats($timeRange = '-1h') {
        // Use simpler query without pivot for aggregate stats
        $flux = '
            from(bucket: "' . $this->config['influxdb']['bucket'] . '")
            |> range(start: ' . $timeRange . ')
            |> filter(fn: (r) => r["_measurement"] == "ping")
            |> filter(fn: (r) => r["_field"] == "rtt_avg" or r["_field"] == "packet_loss" or r["_field"] == "jitter")
            |> group(columns: ["_field"])
            |> mean()
            |> group()
        ';
        
        $rawResults = $this->influx->query($flux);
        
        // Convert to the expected format
        $stats = [];
        foreach ($rawResults as $row) {
            $stats[$row['_field']] = floatval($row['_value'] ?? 0);
        }
        
        return [$stats]; // Return as array to match expected format
    }
    
    public function getHostsWithPacketLoss($timeRange = '-1h', $limit = 10) {
        // Get the most recent packet loss data for each host
        $flux = '
            from(bucket: "' . $this->config['influxdb']['bucket'] . '")
            |> range(start: ' . $timeRange . ')
            |> filter(fn: (r) => r["_measurement"] == "ping")
            |> filter(fn: (r) => r["_field"] == "packet_loss")
            |> filter(fn: (r) => r["_value"] > 0.0)
            |> group(columns: ["host", "organization"])
            |> last()
            |> group()
            |> sort(columns: ["_time"], desc: true)
            |> limit(n: ' . $limit . ')
        ';
        
        $rawResults = $this->influx->query($flux);
        
        // Format the results
        $results = [];
        foreach ($rawResults as $row) {
            $results[] = [
                'organization' => $row['organization'] ?? '',
                'host' => $row['host'] ?? '',
                'packet_loss' => floatval($row['_value'] ?? 0),
                'last_seen' => $row['_time'] ?? ''
            ];
        }
        
        return $results;
    }
    
    public function renderHTML() {
        try {
            $organizations = $this->getOrganizations('-1h'); // Still needed for chart dropdowns
        } catch (Exception $e) {
            $error = $e->getMessage();
        }
        
        include 'dashboard_template.php';
    }
}

// Handle AJAX requests
if (isset($_GET['ajax'])) {
    header('Content-Type: application/json');
    
    try {
        $dashboard = new SmogPingDashboard($config);
        
        switch ($_GET['ajax']) {
            case 'organizations':
                echo json_encode($dashboard->getOrganizations($_GET['range'] ?? '-1h'));
                break;
                
            case 'hosts':
                if (!empty($_GET['org'])) {
                    $hosts = $dashboard->getHostsByOrganization($_GET['org'], $_GET['range'] ?? '-1h');
                    echo json_encode(['hosts' => array_column($hosts, 'host')]);
                } else {
                    echo json_encode(['hosts' => []]);
                }
                break;
                
            case 'hoststats':
                if (!empty($_GET['host']) && !empty($_GET['org'])) {
                    echo json_encode($dashboard->getHostStats($_GET['host'], $_GET['org'], $_GET['range'] ?? '-1h'));
                } else {
                    echo json_encode([]);
                }
                break;
                
            case 'recent':
                echo json_encode($dashboard->getRecentResults($_GET['range'] ?? '-1h', 50));
                break;
                
            case 'timeseries':
                if (!empty($_GET['host']) && !empty($_GET['org'])) {
                    $range = $_GET['range'] ?? '-1h';
                    
                    // Validate range format
                    if (!preg_match('/^-?\d+[mhd]$/', $range)) {
                        echo json_encode(['error' => 'Invalid time range format: ' . $range]);
                        break;
                    }
                    
                    // Ensure range starts with minus sign
                    $range = $range[0] === '-' ? $range : '-' . $range;
                    
                    $data = $dashboard->getHostTimeSeriesStats($_GET['host'], $_GET['org'], $range);
                    echo json_encode(['data' => $data]);
                } else {
                    echo json_encode(['error' => 'Host and organization required']);
                }
                break;
                
            case 'packet_loss_hosts':
                $range = $_GET['range'] ?? '-1h';
                $limit = isset($_GET['limit']) ? intval($_GET['limit']) : $config['dashboard']['items_per_page'];
                
                // Validate range format
                if (!preg_match('/^-?\d+[mhd]$/', $range)) {
                    echo json_encode(['error' => 'Invalid time range format: ' . $range]);
                    break;
                }
                
                // Validate limit (between 1 and 100)
                if ($limit < 1 || $limit > 100) {
                    $limit = $config['dashboard']['items_per_page']; // Default to config value if invalid
                }
                
                // Ensure range starts with minus sign
                $range = $range[0] === '-' ? $range : '-' . $range;
                
                $data = $dashboard->getHostsWithPacketLoss($range, $limit);
                echo json_encode(['data' => $data]);
                break;
                
            default:
                echo json_encode(['error' => 'Unknown AJAX action']);
        }
    } catch (Exception $e) {
        echo json_encode(['error' => $e->getMessage()]);
    }
    exit;
}

// Main dashboard
if (!isset($_GET['ajax'])) {
    $dashboard = new SmogPingDashboard($config);
    $dashboard->renderHTML();
}
?>
