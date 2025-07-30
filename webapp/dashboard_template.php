<?php
// SPDX-License-Identifier: GPL-3.0
// Copyright (C) 2025 FexTel, Inc. <info@ibscale.com>
// Author: James Pearson <jamesp@ibscale.com>
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?= htmlspecialchars($this->config['dashboard']['title']) ?></title>
    <script src="include/chart.js"></script>
    <script src="include/date-fns.min.js"></script>
    <script src="include/chartjs-adapter-date-fns.bundle.min.js"></script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: #f5f5f5;
            color: #333;
            line-height: 1.6;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 1rem 2rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        
        .header h1 {
            font-size: 2rem;
            margin-bottom: 0.5rem;
        }
        
        .header .subtitle {
            opacity: 0.9;
            font-size: 1.1rem;
        }
        
        .controls {
            background: white;
            padding: 1rem 2rem;
            border-bottom: 1px solid #ddd;
            display: flex;
            gap: 1rem;
            align-items: center;
            flex-wrap: wrap;
        }
        
        .control-group {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .control-group label {
            font-weight: 600;
            color: #555;
        }
        
        select, input {
            padding: 0.5rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 0.9rem;
        }
        
        button {
            background: #667eea;
            color: white;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.9rem;
            transition: background-color 0.2s;
        }
        
        button:hover {
            background: #5a6fd8;
        }
        
        .main-content {
            padding: 2rem;
            max-width: 1400px;
            margin: 0 auto;
        }

        .card {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            padding: 1.5rem;
        }
        
        .card h2 {
            color: #667eea;
            margin-bottom: 1rem;
            font-size: 1.4rem;
            border-bottom: 2px solid #f0f0f0;
            padding-bottom: 0.5rem;
        }
        
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 1rem;
            margin: 1rem 0;
        }
        
        .stat-item {
            text-align: center;
            padding: 1rem;
            background: #f8f9fa;
            border-radius: 6px;
            border-left: 4px solid #667eea;
        }
        
        .stat-value {
            font-size: 1.8rem;
            font-weight: bold;
            color: #667eea;
            display: block;
        }
        
        .stat-label {
            font-size: 0.9rem;
            color: #666;
            margin-top: 0.25rem;
        }
        
        .status-good { color: #28a745; font-weight: bold; }
        .status-warning { color: #ffc107; font-weight: bold; }
        .status-critical { color: #dc3545; font-weight: bold; }
        
        .metric-rtt { color: #17a2b8; }
        .metric-loss { color: #fd7e14; }
        .metric-jitter { color: #6f42c1; }
        
        .error {
            background: #f8d7da;
            color: #721c24;
            padding: 1rem;
            border-radius: 4px;
            margin: 1rem 0;
            border: 1px solid #f5c6cb;
        }
        
        padding-bottom: 0.5rem;
        }

        .loading {
            text-align: center;
            padding: 2rem;
            color: #666;
            font-style: italic;
        }
        
        .packet-loss-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        
        .packet-loss-table th,
        .packet-loss-table td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        
        .packet-loss-table th {
            background: #f8f9fa;
            font-weight: 600;
            color: #333;
        }
        
        .packet-loss-value {
            font-weight: bold;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            color: white;
            text-align: center;
            min-width: 60px;
            display: inline-block;
        }
        
        .no-packet-loss {
            text-align: center;
            padding: 2rem;
            color: #28a745;
            font-weight: bold;
        }
        
        .chart-load-btn {
            background: #667eea;
            color: white;
            border: none;
            padding: 0.4rem 0.6rem;
            border-radius: 4px;
            cursor: pointer;
            font-size: 1rem;
            transition: background-color 0.2s;
            min-width: 32px;
            height: 32px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
        }
        
        .chart-load-btn:hover {
            background: #5a6fd8;
            transform: scale(1.1);
        }
        
        .auto-refresh {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .auto-refresh input[type="checkbox"] {
            margin: 0;
        }
        
        .chart-container {
            grid-column: 1 / -1;
            height: 400px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            padding: 1.5rem;
            margin-bottom: 2rem;
        }
        
        @media (max-width: 768px) {
            .dashboard-grid {
                grid-template-columns: 1fr;
            }
            
            .controls {
                flex-direction: column;
                align-items: stretch;
            }
            
            .control-group {
                flex-direction: column;
                align-items: stretch;
            }
            
            .header {
                padding: 1rem;
            }
            
            .main-content {
                padding: 1rem;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1><?= htmlspecialchars($this->config['dashboard']['title']) ?></h1>
        <div class="subtitle">Network Performance Monitoring Dashboard</div>
    </div>

    <div class="main-content">
        <?php if (isset($error)): ?>
            <div class="error">
                <strong>Error:</strong> <?= htmlspecialchars($error) ?>
            </div>
        <?php endif; ?>

        <!-- Chart Controls -->
        <div class="controls">
            <div class="control-group">
                <label for="chart-organization">Organization:</label>
                <select id="chart-organization">
                    <option value="">Select Organization</option>
                </select>
            </div>
            
            <div class="control-group">
                <label for="chart-host">Host:</label>
                <select id="chart-host" disabled>
                    <option value="">Select Host</option>
                </select>
            </div>
            
            <div class="control-group">
                <label for="chart-timerange">Time Range:</label>
                <select id="chart-timerange">
                    <option value="1h" selected>Last 1 Hour</option>
                    <option value="6h">Last 6 Hours</option>
                    <option value="24h">Last 24 Hours</option>
                    <option value="7d">Last 7 Days</option>
                    <option value="30d">Last 30 Days</option>
                </select>
            </div>
            
            <button id="load-chart" disabled>Load Chart</button>
            <button id="refresh-chart" style="margin-left: 10px;" disabled>🔄 Refresh</button>
            
            <div class="auto-refresh">
                <input type="checkbox" id="chart-auto-refresh">
                <label for="chart-auto-refresh">Auto-refresh chart</label>
            </div>
        </div>

        <!-- Charting Section -->
        <div class="card">
            <h2>📈 Ping Chart</h2>
            
            <div class="chart-container" style="position: relative; width: 100%; height: 500px; margin-bottom: 20px; display: none;">
                <canvas id="ping-chart"></canvas>
            </div>
            
            <div class="chart-legend" style="margin-top: 10px; padding: 15px; background: #f8f9fa; border-radius: 5px; display: none;">
                <h4 style="margin: 0 0 10px 0;">Packet Loss Color Guide:</h4>
                <div style="display: flex; align-items: center; gap: 20px; flex-wrap: wrap;">
                    <div style="display: flex; align-items: center; gap: 5px;">
                        <div style="width: 20px; height: 3px; background: rgb(34, 197, 94);"></div>
                        <span>0% Loss</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 5px;">
                        <div style="width: 20px; height: 3px; background: rgb(255, 193, 7);"></div>
                        <span>1% Loss</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 5px;">
                        <div style="width: 20px; height: 3px; background: rgb(201, 122, 120);"></div>
                        <span>25% Loss</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 5px;">
                        <div style="width: 20px; height: 3px; background: rgb(174, 86, 177);"></div>
                        <span>50% Loss</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 5px;">
                        <div style="width: 20px; height: 3px; background: rgb(147, 51, 234);"></div>
                        <span>100% Loss</span>
                    </div>
                </div>
            </div>
        </div>

        <!-- Recent Packet Loss Hosts -->
        <div class="card">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
                <h2 style="margin: 0;">🚨 Recent Packet Loss</h2>
                <div style="display: flex; align-items: center; gap: 10px;">
                    <div class="control-group">
                        <label for="packet-loss-limit">Records:</label>
                        <select id="packet-loss-limit">
                            <option value="10"<?= $this->config['dashboard']['items_per_page'] == 10 ? ' selected' : '' ?>>10</option>
                            <option value="25"<?= $this->config['dashboard']['items_per_page'] == 25 ? ' selected' : '' ?>>25</option>
                            <option value="50"<?= $this->config['dashboard']['items_per_page'] == 50 ? ' selected' : '' ?>>50</option>
                            <option value="75"<?= $this->config['dashboard']['items_per_page'] == 75 ? ' selected' : '' ?>>75</option>
                            <option value="100"<?= $this->config['dashboard']['items_per_page'] == 100 ? ' selected' : '' ?>>100</option>
                        </select>
                    </div>
                    <button id="refresh-alarms" style="background: #667eea; color: white; border: none; padding: 8px 15px; border-radius: 4px; cursor: pointer;">🔄 Refresh Alarms</button>
                </div>
            </div>
            <p>Hosts with packet loss in the selected time range</p>
            
            <div id="packet-loss-hosts">
                <div class="loading">Loading packet loss data...</div>
            </div>
        </div>
    </div>

    <script>
        // Chart functionality
        let pingChart = null;
        let chartAutoRefreshInterval = null;
        
        function initializeChartDropdowns() {
            // Populate chart organization dropdown from server
            const chartOrgSelect = document.getElementById('chart-organization');
            
            fetch('dashboard.php?ajax=organizations')
                .then(response => response.json())
                .then(organizations => {
                    chartOrgSelect.innerHTML = '<option value="">Select Organization</option>';
                    organizations.forEach(org => {
                        chartOrgSelect.add(new Option(org, org));
                    });
                })
                .catch(error => {
                    console.error('Error loading organizations:', error);
                });
            
            // Add event listeners
            chartOrgSelect.addEventListener('change', updateChartHosts);
            document.getElementById('chart-host').addEventListener('change', enableLoadChart);
            document.getElementById('load-chart').addEventListener('click', loadChart);
            document.getElementById('refresh-chart').addEventListener('click', refreshChart);
            document.getElementById('refresh-alarms').addEventListener('click', loadPacketLossHosts);
            document.getElementById('chart-auto-refresh').addEventListener('change', toggleChartAutoRefresh);
        }
        
        // Initialize chart dropdowns on page load
        window.addEventListener('load', function() {
            initializeChartDropdowns();
            loadPacketLossHosts();
            
            // Reload packet loss hosts when time range changes
            document.getElementById('chart-timerange').addEventListener('change', loadPacketLossHosts);
            
            // Reload packet loss hosts when limit changes
            document.getElementById('packet-loss-limit').addEventListener('change', loadPacketLossHosts);
        });
        
        function loadChartForHost(organization, host) {
            // Set the dropdown values
            const orgSelect = document.getElementById('chart-organization');
            const hostSelect = document.getElementById('chart-host');
            
            // Set organization
            orgSelect.value = organization;
            
            // Update hosts for the selected organization
            updateChartHosts().then(() => {
                // Set the host after hosts are loaded
                hostSelect.value = host;
                
                // Enable the load button and load the chart
                enableLoadChart();
                loadChart();
                
                // Scroll to the chart section
                document.querySelector('.chart-container').scrollIntoView({ 
                    behavior: 'smooth', 
                    block: 'start' 
                });
            });
        }
        
        function updateChartHosts() {
            const org = document.getElementById('chart-organization').value;
            const hostSelect = document.getElementById('chart-host');
            const loadButton = document.getElementById('load-chart');
            const refreshButton = document.getElementById('refresh-chart');
            
            // Stop auto-refresh when changing organization
            const autoRefreshCheckbox = document.getElementById('chart-auto-refresh');
            if (autoRefreshCheckbox.checked) {
                autoRefreshCheckbox.checked = false;
                toggleChartAutoRefresh();
            }
            
            hostSelect.innerHTML = '<option value="">Select Host</option>';
            hostSelect.disabled = !org;
            loadButton.disabled = true;
            refreshButton.disabled = true;
            
            if (!org) return Promise.resolve();
            
            // Fetch hosts for selected organization
            return fetch(`dashboard.php?ajax=hosts&org=${encodeURIComponent(org)}`)
                .then(response => response.json())
                .then(data => {
                    if (data.hosts && Array.isArray(data.hosts)) {
                        data.hosts.forEach(host => {
                            hostSelect.add(new Option(host, host));
                        });
                        hostSelect.disabled = false;
                    } else if (data.error) {
                        console.error('Error fetching hosts:', data.error);
                    }
                })
                .catch(error => {
                    console.error('Error fetching hosts:', error);
                });
        }
        
        function enableLoadChart() {
            const org = document.getElementById('chart-organization').value;
            const host = document.getElementById('chart-host').value;
            const loadButton = document.getElementById('load-chart');
            const refreshButton = document.getElementById('refresh-chart');
            
            // Stop auto-refresh when changing host
            const autoRefreshCheckbox = document.getElementById('chart-auto-refresh');
            if (autoRefreshCheckbox.checked) {
                autoRefreshCheckbox.checked = false;
                toggleChartAutoRefresh();
            }
            
            const canLoad = org && host;
            loadButton.disabled = !canLoad;
            refreshButton.disabled = !canLoad;
        }
        
        function refreshChart() {
            // Refresh the current chart if one is loaded
            loadChart();
        }
        
        function toggleChartAutoRefresh() {
            const autoRefreshCheckbox = document.getElementById('chart-auto-refresh');
            const refreshButton = document.getElementById('refresh-chart');
            
            if (autoRefreshCheckbox.checked) {
                // Start auto-refresh (every 30 seconds)
                chartAutoRefreshInterval = setInterval(() => {
                    // Only refresh if a chart is currently loaded
                    const org = document.getElementById('chart-organization').value;
                    const host = document.getElementById('chart-host').value;
                    if (org && host && pingChart) {
                        console.log('Auto-refreshing chart...');
                        loadChart();
                    }
                }, 30000); // 30 seconds
                
                // Update refresh button to show auto-refresh is active
                refreshButton.style.backgroundColor = '#28a745';
                refreshButton.title = 'Auto-refresh enabled (every 30s)';
            } else {
                // Stop auto-refresh
                if (chartAutoRefreshInterval) {
                    clearInterval(chartAutoRefreshInterval);
                    chartAutoRefreshInterval = null;
                }
                
                // Reset refresh button appearance
                refreshButton.style.backgroundColor = '#667eea';
                refreshButton.title = 'Refresh chart';
            }
        }
        
        function loadChart() {
            const org = document.getElementById('chart-organization').value;
            const host = document.getElementById('chart-host').value;
            const timeRange = document.getElementById('chart-timerange').value;
            
            if (!org || !host) return;
            
            const loadButton = document.getElementById('load-chart');
            const refreshButton = document.getElementById('refresh-chart');
            loadButton.disabled = true;
            refreshButton.disabled = true;
            loadButton.textContent = 'Loading...';
            refreshButton.textContent = '🔄 Refreshing...';
            
            // Convert time range to InfluxDB format (add minus sign if not present)
            const influxTimeRange = timeRange.startsWith('-') ? timeRange : '-' + timeRange;
            
            // Fetch time series data
            const params = new URLSearchParams({
                ajax: 'timeseries',
                org: org,
                host: host,
                range: influxTimeRange
            });
            
            fetch(`dashboard.php?${params}`)
                .then(response => response.json())
                .then(data => {
                    console.log('Chart data response:', data); // Debug logging
                    
                    if (data.error) {
                        alert('Error loading chart data: ' + data.error);
                        console.error('Chart data error:', data.error);
                        return;
                    }
                    
                    if (!data.data || !Array.isArray(data.data) || data.data.length === 0) {
                        alert('No data found for the selected host and time range');
                        return;
                    }
                    
                    renderChart(data.data, org, host);
                    document.querySelector('.chart-container').style.display = 'block';
                    document.querySelector('.chart-legend').style.display = 'block';
                })
                .catch(error => {
                    console.error('Error loading chart data:', error);
                    alert('Error loading chart data: ' + error.message);
                })
                .finally(() => {
                    const refreshButton = document.getElementById('refresh-chart');
                    loadButton.disabled = false;
                    refreshButton.disabled = false;
                    loadButton.textContent = 'Load Chart';
                    refreshButton.textContent = '🔄 Refresh';
                });
        }
        
        function getPacketLossColor(lossPercent) {
            // Color scheme: Green (0%) → Yellow (1%+) → Purple (100%)
            if (lossPercent === 0) {
                // Pure green for 0% packet loss
                return 'rgb(34, 197, 94)';
            }
            
            // For any packet loss > 0%, interpolate from yellow to purple
            // Yellow: rgb(255, 193, 7) to Purple: rgb(147, 51, 234)
            const normalizedLoss = Math.min(lossPercent / 100, 1); // Ensure max is 1
            
            // Use square root curve for better sensitivity at lower values
            const curvedLoss = Math.pow(normalizedLoss, 0.6);
            
            const startRed = 255;   // Yellow start
            const startGreen = 193;
            const startBlue = 7;
            
            const endRed = 147;     // Purple end
            const endGreen = 51;
            const endBlue = 234;
            
            const red = Math.round(startRed + (endRed - startRed) * curvedLoss);
            const green = Math.round(startGreen + (endGreen - startGreen) * curvedLoss);
            const blue = Math.round(startBlue + (endBlue - startBlue) * curvedLoss);
            
            return `rgb(${red}, ${green}, ${blue})`;
        }
        
        function renderChart(data, org, host) {
            const ctx = document.getElementById('ping-chart').getContext('2d');
            
            if (pingChart) {
                pingChart.destroy();
            }
            
            // Prepare chart data with color segments
            const chartData = data.map(point => ({
                x: new Date(point._time),
                y: parseFloat(point.rtt_avg || 0),
                packetLoss: parseFloat(point.packet_loss || 0),
                jitter: parseFloat(point.jitter || 0)
            })).filter(point => !isNaN(point.y) && !isNaN(point.packetLoss));
            
            if (chartData.length === 0) {
                alert('No valid data points found for charting');
                return;
            }
            
            // Prepare jitter bar data (error bars)
            const jitterData = chartData.map(point => ({
                x: point.x,
                y: [point.y - point.jitter/2, point.y + point.jitter/2] // Error bar range
            }));
            
            pingChart = new Chart(ctx, {
                type: 'line',
                data: {
                    datasets: [
                        {
                            label: 'Jitter',
                            type: 'bar',
                            data: jitterData,
                            backgroundColor: 'rgba(128, 128, 128, 0.3)',
                            borderColor: 'rgba(128, 128, 128, 0.6)',
                            borderWidth: 1,
                            barThickness: 2,
                            categoryPercentage: 1.0,
                            barPercentage: 1.0,
                            order: 2,
                            parsing: {
                                yAxisKey: 'y'
                            }
                        },
                        {
                            label: 'Ping RTT (ms)',
                            data: chartData,
                            borderColor: function(ctx) {
                                if (ctx.dataIndex !== undefined && chartData[ctx.dataIndex]) {
                                    return getPacketLossColor(chartData[ctx.dataIndex].packetLoss);
                                }
                                return 'rgb(34, 197, 94)'; // Default green
                            },
                            backgroundColor: function(ctx) {
                                if (ctx.dataIndex !== undefined && chartData[ctx.dataIndex]) {
                                    return getPacketLossColor(chartData[ctx.dataIndex].packetLoss) + '20';
                                }
                                return 'rgba(34, 197, 94, 0.2)'; // Default green with transparency
                            },
                            borderWidth: 3, // Slightly thicker for better visibility
                            fill: false,
                            tension: 0.1,
                            pointRadius: 4,
                            pointHoverRadius: 6,
                            order: 1,
                            segment: {
                                borderColor: function(ctx) {
                                    // Use the higher packet loss value between the two points for the segment
                                    if (ctx.p0DataIndex !== undefined && ctx.p1DataIndex !== undefined && 
                                        chartData[ctx.p0DataIndex] && chartData[ctx.p1DataIndex]) {
                                        const loss1 = chartData[ctx.p0DataIndex].packetLoss;
                                        const loss2 = chartData[ctx.p1DataIndex].packetLoss;
                                        const maxLoss = Math.max(loss1, loss2);
                                        return getPacketLossColor(maxLoss);
                                    }
                                    return 'rgb(34, 197, 94)'; // Default green
                                },
                                borderWidth: function(ctx) {
                                    // Make segments with packet loss thicker for emphasis
                                    if (ctx.p0DataIndex !== undefined && ctx.p1DataIndex !== undefined && 
                                        chartData[ctx.p0DataIndex] && chartData[ctx.p1DataIndex]) {
                                        const loss1 = chartData[ctx.p0DataIndex].packetLoss;
                                        const loss2 = chartData[ctx.p1DataIndex].packetLoss;
                                        const maxLoss = Math.max(loss1, loss2);
                                        return maxLoss > 0 ? 4 : 2; // Thicker line for packet loss segments
                                    }
                                    return 2;
                                }
                            }
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        title: {
                            display: true,
                            text: `Ping Times - ${org} / ${host}`,
                            font: { size: 16 }
                        },
                        legend: {
                            display: false
                        },
                        tooltip: {
                            mode: 'index',
                            intersect: false,
                            callbacks: {
                                label: function(context) {
                                    if (context.datasetIndex === 0) {
                                        // Jitter dataset - don't show in tooltip
                                        return null;
                                    }
                                    const point = chartData[context.dataIndex];
                                    return [
                                        `RTT: ${context.parsed.y.toFixed(1)}ms`,
                                        `Packet Loss: ${point.packetLoss.toFixed(1)}%`,
                                        `Jitter: ${point.jitter.toFixed(1)}ms`
                                    ];
                                },
                                filter: function(tooltipItem) {
                                    // Hide jitter dataset from tooltip
                                    return tooltipItem.datasetIndex !== 0;
                                }
                            }
                        }
                    },
                    scales: {
                        x: {
                            type: 'time',
                            time: {
                                displayFormats: {
                                    hour: 'MMM dd HH:mm',
                                    day: 'MMM dd'
                                }
                            },
                            title: {
                                display: true,
                                text: 'Time'
                            }
                        },
                        y: {
                            title: {
                                display: true,
                                text: 'RTT (ms)'
                            },
                            beginAtZero: true
                        }
                    },
                    interaction: {
                        mode: 'nearest',
                        axis: 'x',
                        intersect: false
                    }
                }
            });
        }
        
        function loadPacketLossHosts() {
            const timeRange = document.getElementById('chart-timerange').value;
            const limit = document.getElementById('packet-loss-limit').value;
            const container = document.getElementById('packet-loss-hosts');
            const refreshButton = document.getElementById('refresh-alarms');
            
            // Visual feedback for refresh button
            const originalText = refreshButton.textContent;
            refreshButton.disabled = true;
            refreshButton.textContent = '🔄 Refreshing...';
            
            container.innerHTML = '<div class="loading">Loading packet loss data...</div>';
            
            fetch(`dashboard.php?ajax=packet_loss_hosts&range=${timeRange}&limit=${limit}`)
                .then(response => response.json())
                .then(result => {
                    if (result.error) {
                        container.innerHTML = `<div class="error">Error: ${result.error}</div>`;
                        return;
                    }
                    
                    const hosts = result.data;
                    
                    if (hosts.length === 0) {
                        container.innerHTML = '<div class="no-packet-loss">✅ No hosts with packet loss in the selected time range</div>';
                        return;
                    }
                    
                    let html = `
                        <table class="packet-loss-table">
                            <thead>
                                <tr>
                                    <th>Organization</th>
                                    <th>Host</th>
                                    <th>Packet Loss</th>
                                    <th>Last Seen</th>
                                    <th>Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                    `;
                    
                    hosts.forEach(host => {
                        const packetLoss = parseFloat(host.packet_loss);
                        const color = getPacketLossColor(packetLoss);
                        const lastSeen = new Date(host.last_seen).toLocaleString();
                        
                        html += `
                            <tr>
                                <td>${host.organization}</td>
                                <td>${host.host}</td>
                                <td>
                                    <span class="packet-loss-value" style="background-color: ${color}">
                                        ${packetLoss.toFixed(1)}%
                                    </span>
                                </td>
                                <td>${lastSeen}</td>
                                <td>
                                    <button class="chart-load-btn" onclick="loadChartForHost('${host.organization}', '${host.host}')" title="Load chart for this host">
                                        🔍
                                    </button>
                                </td>
                            </tr>
                        `;
                    });
                    
                    html += '</tbody></table>';
                    container.innerHTML = html;
                })
                .catch(error => {
                    container.innerHTML = `<div class="error">Error loading packet loss data: ${error.message}</div>`;
                })
                .finally(() => {
                    // Reset refresh button
                    refreshButton.disabled = false;
                    refreshButton.textContent = originalText;
                });
        }
    </script>
</body>
</html>
