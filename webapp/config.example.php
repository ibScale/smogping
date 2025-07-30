<?php
// SPDX-License-Identifier: GPL-3.0
// Copyright (C) 2025 FexTel, Inc. <info@ibscale.com>
// Author: James Pearson <jamesp@ibscale.com>

/**
 * SmogPing Dashboard Configuration
 * Copy this file to config.php and customize for your environment
 */

// Prevent direct access
if (!defined('SMOGPING_DASHBOARD')) {
    http_response_code(403);
    exit('Access denied');
}

return [
    'influxdb' => [
        'url' => 'http://localhost:8086',
        'token' => 'YOUR_INFLUXDB_TOKEN_HERE',
        'org' => 'YOUR_ORGANIZATION_NAME',
        'bucket' => 'YOUR_BUCKET_NAME'
    ],
    'dashboard' => [
        'title' => 'SmogPing Network Monitor',
        'refresh_interval' => 30, // Auto-refresh interval in seconds
        'default_timerange' => '-1h', // Default time range (-5m, -1h, -6h, -24h, -7d)
        'items_per_page' => 10 // Number of recent packet loss results to show (10, 25, 50, 75, 100)
    ]
];
?>
