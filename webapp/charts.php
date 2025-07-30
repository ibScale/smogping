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
    <title>SmogPing Charts - Network Performance Analysis</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/date-fns@2.29.3/index.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/chartjs-adapter-date-fns@2.0.0/dist/chartjs-adapter-date-fns.bundle.min.js"></script>
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
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 1rem 2rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
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
        
        select, button {
            padding: 0.5rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 0.9rem;
        }
        
        button {
            background: #667eea;
            color: white;
            border: none;
            cursor: pointer;
        }
        
        .main-content {
            padding: 2rem;
            max-width: 1400px;
            margin: 0 auto;
        }
        
        .charts-grid {
            display: grid;
            grid-template-columns: 1fr;
            gap: 2rem;
        }
        
        .chart-card {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            padding: 1.5rem;
        }
        
        .chart-card h2 {
            color: #667eea;
            margin-bottom: 1rem;
            font-size: 1.4rem;
        }
        
        .chart-container {
            position: relative;
            height: 400px;
        }
        
        .loading {
            text-align: center;
            padding: 2rem;
            color: #666;
        }
        
        .error {
            background: #f8d7da;
            color: #721c24;
            padding: 1rem;
            border-radius: 4px;
            margin: 1rem 0;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>📊 SmogPing Charts</h1>
        <div>Network Performance Analysis & Trends</div>
    </div>

    <div class="controls">
        <div class="control-group">
            <label for="timeRange">Time Range:</label>
            <select id="timeRange">
                <option value="-1h">Last Hour</option>
                <option value="-6h">Last 6 Hours</option>
                <option value="-24h">Last 24 Hours</option>
                <option value="-7d">Last 7 Days</option>
                <option value="-30d">Last 30 Days</option>
            </select>
        </div>
        
        <div class="control-group">
            <label for="organization">Organization:</label>
            <select id="organization">
                <option value="">All Organizations</option>
            </select>
        </div>
        
        <div class="control-group">
            <label for="host">Host:</label>
            <select id="host">
                <option value="">All Hosts</option>
            </select>
        </div>
        
        <button onclick="updateCharts()">Update Charts</button>
        <button onclick="window.location.href='dashboard.php'">Back to Dashboard</button>
    </div>

    <div class="main-content">
        <div id="error" class="error" style="display: none;"></div>
        
        <div class="charts-grid">
            <div class="chart-card">
                <h2>Round Trip Time (RTT)</h2>
                <div class="chart-container">
                    <canvas id="rttChart"></canvas>
                </div>
            </div>
            
            <div class="chart-card">
                <h2>Packet Loss</h2>
                <div class="chart-container">
                    <canvas id="lossChart"></canvas>
                </div>
            </div>
            
            <div class="chart-card">
                <h2>Jitter</h2>
                <div class="chart-container">
                    <canvas id="jitterChart"></canvas>
                </div>
            </div>
            
            <div class="chart-card">
                <h2>Combined Performance Overview</h2>
                <div class="chart-container">
                    <canvas id="combinedChart"></canvas>
                </div>
            </div>
        </div>
    </div>

    <script>
        let charts = {};
        let currentData = [];
        
        // Initialize charts
        function initCharts() {
            const commonOptions = {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            displayFormats: {
                                minute: 'HH:mm',
                                hour: 'MM/dd HH:mm',
                                day: 'MM/dd'
                            }
                        },
                        title: {
                            display: true,
                            text: 'Time'
                        }
                    }
                },
                plugins: {
                    legend: {
                        position: 'top'
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false
                    }
                },
                interaction: {
                    mode: 'nearest',
                    axis: 'x',
                    intersect: false
                }
            };
            
            // RTT Chart
            charts.rtt = new Chart(document.getElementById('rttChart'), {
                type: 'line',
                data: { datasets: [] },
                options: {
                    ...commonOptions,
                    scales: {
                        ...commonOptions.scales,
                        y: {
                            title: {
                                display: true,
                                text: 'Round Trip Time (ms)'
                            },
                            beginAtZero: true
                        }
                    }
                }
            });
            
            // Loss Chart
            charts.loss = new Chart(document.getElementById('lossChart'), {
                type: 'line',
                data: { datasets: [] },
                options: {
                    ...commonOptions,
                    scales: {
                        ...commonOptions.scales,
                        y: {
                            title: {
                                display: true,
                                text: 'Packet Loss (%)'
                            },
                            beginAtZero: true,
                            max: 100
                        }
                    }
                }
            });
            
            // Jitter Chart
            charts.jitter = new Chart(document.getElementById('jitterChart'), {
                type: 'line',
                data: { datasets: [] },
                options: {
                    ...commonOptions,
                    scales: {
                        ...commonOptions.scales,
                        y: {
                            title: {
                                display: true,
                                text: 'Jitter (ms)'
                            },
                            beginAtZero: true
                        }
                    }
                }
            });
            
            // Combined Chart
            charts.combined = new Chart(document.getElementById('combinedChart'), {
                type: 'line',
                data: { datasets: [] },
                options: {
                    ...commonOptions,
                    scales: {
                        ...commonOptions.scales,
                        y: {
                            type: 'linear',
                            display: true,
                            position: 'left',
                            title: {
                                display: true,
                                text: 'RTT & Jitter (ms)'
                            }
                        },
                        y1: {
                            type: 'linear',
                            display: true,
                            position: 'right',
                            title: {
                                display: true,
                                text: 'Packet Loss (%)'
                            },
                            max: 100,
                            grid: {
                                drawOnChartArea: false
                            }
                        }
                    }
                }
            });
        }
        
        // Load organizations
        async function loadOrganizations() {
            try {
                const response = await fetch('dashboard.php?ajax=organizations&range=-24h');
                const orgs = await response.json();
                
                const select = document.getElementById('organization');
                select.innerHTML = '<option value="">All Organizations</option>';
                
                orgs.forEach(org => {
                    const option = document.createElement('option');
                    option.value = org;
                    option.textContent = org;
                    select.appendChild(option);
                });
            } catch (error) {
                console.error('Error loading organizations:', error);
            }
        }
        
        // Load hosts for selected organization
        async function loadHosts() {
            const org = document.getElementById('organization').value;
            const hostSelect = document.getElementById('host');
            
            if (!org) {
                hostSelect.innerHTML = '<option value="">All Hosts</option>';
                return;
            }
            
            try {
                const timeRange = document.getElementById('timeRange').value;
                const response = await fetch(`dashboard.php?ajax=hosts&org=${encodeURIComponent(org)}&range=${timeRange}`);
                const hosts = await response.json();
                
                hostSelect.innerHTML = '<option value="">All Hosts</option>';
                hosts.forEach(host => {
                    const option = document.createElement('option');
                    option.value = host.host;
                    option.textContent = `${host.host} (${host.ip})`;
                    hostSelect.appendChild(option);
                });
            } catch (error) {
                console.error('Error loading hosts:', error);
            }
        }
        
        // Load chart data
        async function loadChartData() {
            const timeRange = document.getElementById('timeRange').value;
            const org = document.getElementById('organization').value;
            const host = document.getElementById('host').value;
            
            try {
                let url = `dashboard.php?ajax=recent&range=${timeRange}&limit=1000`;
                if (org) url += `&org=${encodeURIComponent(org)}`;
                if (host) url += `&host=${encodeURIComponent(host)}`;
                
                const response = await fetch(url);
                const data = await response.json();
                
                if (data.error) {
                    throw new Error(data.error);
                }
                
                currentData = data;
                updateChartData();
                document.getElementById('error').style.display = 'none';
                
            } catch (error) {
                console.error('Error loading chart data:', error);
                const errorDiv = document.getElementById('error');
                errorDiv.textContent = 'Error loading data: ' + error.message;
                errorDiv.style.display = 'block';
            }
        }
        
        // Update chart data
        function updateChartData() {
            // Group data by host
            const hostGroups = {};
            
            currentData.forEach(point => {
                const hostKey = `${point.organization || 'Unknown'} - ${point.host || 'Unknown'}`;
                
                if (!hostGroups[hostKey]) {
                    hostGroups[hostKey] = {
                        rtt: [],
                        loss: [],
                        jitter: []
                    };
                }
                
                const timestamp = new Date(point._time);
                
                hostGroups[hostKey].rtt.push({
                    x: timestamp,
                    y: parseFloat(point.rtt_avg || 0)
                });
                
                hostGroups[hostKey].loss.push({
                    x: timestamp,
                    y: parseFloat(point.packet_loss || 0)
                });
                
                hostGroups[hostKey].jitter.push({
                    x: timestamp,
                    y: parseFloat(point.jitter || 0)
                });
            });
            
            // Generate colors for hosts
            const colors = [
                '#667eea', '#764ba2', '#f093fb', '#f5576c',
                '#4facfe', '#00f2fe', '#43e97b', '#38f9d7',
                '#ffecd2', '#fcb69f', '#a8edea', '#fed6e3'
            ];
            
            const hostKeys = Object.keys(hostGroups);
            
            // Update RTT Chart
            charts.rtt.data.datasets = hostKeys.map((hostKey, index) => ({
                label: hostKey,
                data: hostGroups[hostKey].rtt,
                borderColor: colors[index % colors.length],
                backgroundColor: colors[index % colors.length] + '20',
                fill: false,
                tension: 0.1
            }));
            charts.rtt.update();
            
            // Update Loss Chart
            charts.loss.data.datasets = hostKeys.map((hostKey, index) => ({
                label: hostKey,
                data: hostGroups[hostKey].loss,
                borderColor: colors[index % colors.length],
                backgroundColor: colors[index % colors.length] + '20',
                fill: false,
                tension: 0.1
            }));
            charts.loss.update();
            
            // Update Jitter Chart
            charts.jitter.data.datasets = hostKeys.map((hostKey, index) => ({
                label: hostKey,
                data: hostGroups[hostKey].jitter,
                borderColor: colors[index % colors.length],
                backgroundColor: colors[index % colors.length] + '20',
                fill: false,
                tension: 0.1
            }));
            charts.jitter.update();
            
            // Update Combined Chart (first host only for clarity)
            if (hostKeys.length > 0) {
                const firstHost = hostKeys[0];
                charts.combined.data.datasets = [
                    {
                        label: `${firstHost} - RTT`,
                        data: hostGroups[firstHost].rtt,
                        borderColor: colors[0],
                        backgroundColor: colors[0] + '20',
                        yAxisID: 'y',
                        fill: false,
                        tension: 0.1
                    },
                    {
                        label: `${firstHost} - Jitter`,
                        data: hostGroups[firstHost].jitter,
                        borderColor: colors[1],
                        backgroundColor: colors[1] + '20',
                        yAxisID: 'y',
                        fill: false,
                        tension: 0.1
                    },
                    {
                        label: `${firstHost} - Loss`,
                        data: hostGroups[firstHost].loss,
                        borderColor: colors[2],
                        backgroundColor: colors[2] + '20',
                        yAxisID: 'y1',
                        fill: false,
                        tension: 0.1
                    }
                ];
            } else {
                charts.combined.data.datasets = [];
            }
            charts.combined.update();
        }
        
        // Update all charts
        async function updateCharts() {
            await loadChartData();
        }
        
        // Initialize page
        window.addEventListener('load', async function() {
            initCharts();
            await loadOrganizations();
            await loadChartData();
            
            // Set up event listeners
            document.getElementById('organization').addEventListener('change', loadHosts);
        });
    </script>
</body>
</html>
