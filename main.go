// SPDX-License-Identifier: GPL-3.0
// Copyright (C) 2025 FexTel, Inc. <info@ibscale.com>
// Author: James Pearson <jamesp@ibscale.com>

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"math"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	probing "github.com/prometheus-community/pro-bing"
)

// Object pools for memory efficiency
var (
	pingResultPool = sync.Pool{
		New: func() interface{} {
			return &PingResult{}
		},
	}

	rttSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]time.Duration, 0, 10) // Pre-allocate for typical ping count
		},
	}
)

// Config represents the main configuration structure
type Config struct {
	InfluxURL          string `toml:"influx_url"`
	InfluxToken        string `toml:"influx_token"`
	InfluxOrg          string `toml:"influx_org"`
	InfluxBucket       string `toml:"influx_bucket"`
	InfluxBatchSize    int    `toml:"influx_batch_size"`
	InfluxBatchTime    int    `toml:"influx_batch_time"`
	DataPointPings     int    `toml:"data_point_pings"`
	DataPointTime      int    `toml:"data_point_time"`
	PingTimeout        int    `toml:"ping_timeout"`
	PingSource         string `toml:"ping_source"`
	DNSRefresh         int    `toml:"dns_refresh"`
	AlarmRate          int    `toml:"alarm_rate"`
	AlarmReceiver      string `toml:"alarm_receiver"`
	MaxConcurrentPings int    `toml:"max_concurrent_pings"`
}

// Host represents a target host to ping
type Host struct {
	Name          string `toml:"name"`
	IP            string `toml:"ip"`
	AlarmPing     int    `toml:"alarmping"`
	AlarmLoss     int    `toml:"alarmloss"`
	AlarmJitter   int    `toml:"alarmjitter"`
	AlarmReceiver string `toml:"alarmreceiver"`
	PingSource    string `toml:"pingsource"`
	// DNS resolution fields (not in TOML)
	ResolvedIP   string    `toml:"-"` // Current resolved IP address
	LastDNSCheck time.Time `toml:"-"` // Last time DNS was checked
	IsDNSName    bool      `toml:"-"` // True if IP field contains a DNS name
}

// DNSCache represents a DNS resolution cache entry
type DNSCache struct {
	Hostname    string
	ResolvedIP  string
	LastChecked time.Time
	DNSChanges  int // Counter for DNS changes
}

// DNSResolver handles DNS resolution and caching
type DNSResolver struct {
	cache    map[string]*DNSCache
	cacheMux sync.RWMutex
	resolver *net.Resolver
}

// Organization represents a group of hosts
type Organization struct {
	Hosts []Host `toml:"hosts"`
}

// TargetsConfig represents the targets configuration structure
type TargetsConfig struct {
	Include       []string                `toml:"include"`
	Organizations map[string]Organization `toml:"organizations"`
}

// PingResult represents the result of a ping operation
type PingResult struct {
	Host       Host
	AvgRTT     time.Duration
	PacketLoss float64
	Jitter     time.Duration
	Timestamp  time.Time
	OrgName    string
}

// TargetInfo represents a target with its organization context
type TargetInfo struct {
	Host    Host
	OrgName string
}

// TOML Validation Error Types
type TOMLValidationError struct {
	File    string
	Field   string
	Value   interface{}
	Message string
	Line    int
}

func (e *TOMLValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("TOML validation error in %s (line %d): %s = %v - %s",
			e.File, e.Line, e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("TOML validation error in %s: %s = %v - %s",
		e.File, e.Field, e.Value, e.Message)
}

type TOMLParseError struct {
	File    string
	Line    int
	Column  int
	Message string
	Context string
}

func (e *TOMLParseError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("TOML parse error in %s at line %d, column %d: %s",
			e.File, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("TOML parse error in %s: %s", e.File, e.Message)
}

// ConfigValidator handles configuration validation
type ConfigValidator struct {
	errors   []error
	warnings []string
}

func (cv *ConfigValidator) AddError(err error) {
	cv.errors = append(cv.errors, err)
}

func (cv *ConfigValidator) AddWarning(msg string) {
	cv.warnings = append(cv.warnings, msg)
}

func (cv *ConfigValidator) HasErrors() bool {
	return len(cv.errors) > 0
}

func (cv *ConfigValidator) GetErrors() []error {
	return cv.errors
}

func (cv *ConfigValidator) GetWarnings() []string {
	return cv.warnings
}

// PingJob represents a ping job for the worker pool
type PingJob struct {
	OrgName string
	Host    Host
}

// PingWorker represents a worker that processes ping jobs
type PingWorker struct {
	id       int
	app      *SmogPing
	jobQueue <-chan PingJob
	quit     chan bool
}

// PingWorkerPool manages a pool of ping workers
type PingWorkerPool struct {
	workers    []*PingWorker
	jobQueue   chan PingJob
	resultChan chan *PingResult
	quit       chan bool
	wg         sync.WaitGroup
}

// SmogPing represents the main application
type SmogPing struct {
	config      Config
	targets     TargetsConfig
	influxWrite api.WriteAPI
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	// Worker pool components (replacing semaphore)
	workerPool *PingWorkerPool
	// DNS resolution components
	dnsResolver *DNSResolver
	// Batching components
	batchMutex  sync.Mutex
	batchPoints []*write.Point
	lastFlush   time.Time
	// Alarm components
	lastAlarms map[string]time.Time // Track last alarm time per host
	alarmMutex sync.RWMutex         // Protect alarm tracking
	// CLI flags
	verbose     bool   // Verbose output
	debug       bool   // Debug output
	noAlarm     bool   // Disable alarm system
	noLog       bool   // Disable alarm logging to syslog
	configFile  string // Path to config file
	targetsFile string // Path to targets file
	// Syslog writer
	syslogWriter *syslog.Writer // Syslog writer for structured logging
	// File watching
	watcher    *fsnotify.Watcher // File system watcher
	targetsMux sync.RWMutex      // Protects targets during reload
	reloadChan chan bool         // Channel to signal configuration reload
}

func main() {
	app := &SmogPing{}

	// Parse command line flags
	app.parseFlags()

	// Setup syslog
	app.setupSyslog()

	// Load configuration
	if err := app.loadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Load targets
	if err := app.loadTargets(); err != nil {
		log.Fatalf("Failed to load targets: %v", err)
	}

	// Setup DNS resolver
	app.setupDNSResolver()

	// Perform DNS pre-flight checks
	if err := app.performDNSPreflightChecks(); err != nil {
		log.Fatalf("DNS pre-flight checks failed: %v", err)
	}

	// Validate configuration sanity
	if err := app.validateConfiguration(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Setup InfluxDB
	if err := app.setupInfluxDB(); err != nil {
		log.Fatalf("Failed to setup InfluxDB: %v", err)
	}

	// Setup context for graceful shutdown
	app.ctx, app.cancel = context.WithCancel(context.Background())

	// Setup worker pool (replaces optimization components)
	// app.setupWorkerPool() // Disabled - using individual ping schedules instead

	// Setup batching
	app.setupBatching()

	// Setup alarm system (unless disabled)
	if !app.noAlarm {
		app.setupAlarms()
	} else {
		log.Println("Alarm system disabled by --noalarm flag")
	}

	// Setup file watching for target changes
	if err := app.setupFileWatching(); err != nil {
		log.Printf("Warning: Failed to setup file watching: %v", err)
	}

	// Start DNS refresh monitoring
	app.startDNSRefreshMonitoring()

	// Start ping monitoring
	app.startPingMonitoring()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	app.cancel()

	// Stop worker pool
	if app.workerPool != nil {
		app.stopWorkerPool()
	}

	app.wg.Wait()
	log.Println("Shutdown complete")

	// Close file watcher
	if app.watcher != nil {
		app.watcher.Close()
	}

	// Close syslog
	if app.syslogWriter != nil {
		app.syslogWriter.Close()
	}
}

// parseFlags parses command line flags
func (sp *SmogPing) parseFlags() {
	flag.BoolVar(&sp.verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&sp.verbose, "v", false, "Enable verbose output (short)")
	flag.BoolVar(&sp.debug, "debug", false, "Enable debug output")
	flag.BoolVar(&sp.debug, "d", false, "Enable debug output (short)")
	flag.BoolVar(&sp.noAlarm, "noalarm", false, "Disable alarm system")
	flag.BoolVar(&sp.noLog, "nolog", false, "Disable alarm logging to syslog")
	flag.StringVar(&sp.configFile, "config", "config.toml", "Path to configuration file")
	flag.StringVar(&sp.configFile, "c", "config.toml", "Path to configuration file (short)")
	flag.StringVar(&sp.targetsFile, "targets", "targets.toml", "Path to targets file")
	flag.StringVar(&sp.targetsFile, "t", "targets.toml", "Path to targets file (short)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "SmogPing - Network monitoring with InfluxDB storage\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDefault configuration files:\n")
		fmt.Fprintf(os.Stderr, "  config.toml          Main configuration file (copy from config.default.toml)\n")
		fmt.Fprintf(os.Stderr, "  config.default.toml  Default configuration template (do not modify)\n")
		fmt.Fprintf(os.Stderr, "  targets.toml         Target hosts to monitor (can include other files)\n")
	}

	flag.Parse()

	// Set log flags based on options
	if sp.debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		sp.verbose = true // Debug implies verbose
		log.Println("Debug mode enabled")
	} else if sp.verbose {
		log.SetFlags(log.LstdFlags)
		log.Println("Verbose mode enabled")
	} else {
		log.SetFlags(log.LstdFlags)
	}
}

// setupSyslog initializes syslog writer for structured logging
func (sp *SmogPing) setupSyslog() {
	var err error
	sp.syslogWriter, err = syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "smogping")
	if err != nil {
		log.Printf("Warning: failed to initialize syslog: %v", err)
		sp.syslogWriter = nil
	}
}

// syslogInfo logs informational messages to syslog
func (sp *SmogPing) syslogInfo(format string, args ...interface{}) {
	if sp.syslogWriter != nil {
		sp.syslogWriter.Info(fmt.Sprintf(format, args...))
	}
}

// syslogWarning logs warning messages to syslog
func (sp *SmogPing) syslogWarning(format string, args ...interface{}) {
	if sp.syslogWriter != nil {
		sp.syslogWriter.Warning(fmt.Sprintf(format, args...))
	}
}

// debugf logs debug messages if debug mode is enabled
func (sp *SmogPing) debugf(format string, args ...interface{}) {
	if sp.debug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// verbosef logs verbose messages if verbose mode is enabled
func (sp *SmogPing) verbosef(format string, args ...interface{}) {
	if sp.verbose {
		log.Printf("[VERBOSE] "+format, args...)
	}
}

// loadConfig loads configuration from specified config file
func (sp *SmogPing) loadConfig() error {
	sp.debugf("Loading configuration file: %s", sp.configFile)

	// Check if config file exists
	if _, err := os.Stat(sp.configFile); err != nil {
		return fmt.Errorf("%s not found - please ensure the config file exists: %w", sp.configFile, err)
	}

	// Load config file with validation
	if err := sp.loadAndValidateConfigFile(sp.configFile, &sp.config, true); err != nil {
		return fmt.Errorf("failed to load %s: %w", sp.configFile, err)
	}
	sp.debugf("Loaded and validated %s", sp.configFile)

	sp.verbosef("Loaded configuration: InfluxDB=%s, Org=%s, Bucket=%s",
		sp.config.InfluxURL, sp.config.InfluxOrg, sp.config.InfluxBucket)

	return nil
}

// loadAndValidateConfigFile loads a TOML config file with comprehensive validation
func (sp *SmogPing) loadAndValidateConfigFile(filename string, config *Config, isDefault bool) error {
	// Check if file exists and is readable
	if _, err := os.Stat(filename); err != nil {
		if isDefault {
			return fmt.Errorf("required config file %s not found: %w", filename, err)
		}
		return fmt.Errorf("config file %s not accessible: %w", filename, err)
	}

	// Check file permissions and size
	if err := sp.validateFileBasics(filename); err != nil {
		return fmt.Errorf("file validation failed for %s: %w", filename, err)
	}

	// Parse TOML with detailed error handling
	metadata, err := toml.DecodeFile(filename, config)
	if err != nil {
		return sp.enhanceTOMLError(filename, err)
	}

	// Validate TOML structure and unknown fields
	if err := sp.validateTOMLStructure(filename, metadata, isDefault); err != nil {
		return err
	}

	// Validate config field values
	if err := sp.validateConfigFields(filename, config, isDefault); err != nil {
		return err
	}

	sp.debugf("Successfully loaded and validated %s", filename)
	return nil
}

// validateFileBasics performs basic file validation
func (sp *SmogPing) validateFileBasics(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}

	// Check file size (reasonable limits)
	if info.Size() == 0 {
		return fmt.Errorf("file is empty")
	}
	if info.Size() > 1024*1024 { // 1MB limit
		return fmt.Errorf("file too large (%d bytes), maximum 1MB", info.Size())
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}

	// Check read permissions
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}
	file.Close()

	return nil
}

// enhanceTOMLError provides detailed TOML parsing error information
func (sp *SmogPing) enhanceTOMLError(filename string, err error) error {
	errStr := err.Error()

	// Extract line and column information from TOML error
	lineRegex := regexp.MustCompile(`line (\d+)`)
	columnRegex := regexp.MustCompile(`column (\d+)`)

	var line, column int
	if matches := lineRegex.FindStringSubmatch(errStr); len(matches) > 1 {
		if l, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
			line = l
		}
	}
	if matches := columnRegex.FindStringSubmatch(errStr); len(matches) > 1 {
		if c, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
			column = c
		}
	}

	// Provide context around the error
	context := sp.getFileContext(filename, line)

	return &TOMLParseError{
		File:    filename,
		Line:    line,
		Column:  column,
		Message: errStr,
		Context: context,
	}
}

// getFileContext reads lines around an error for better context
func (sp *SmogPing) getFileContext(filename string, lineNum int) string {
	if lineNum <= 0 {
		return ""
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	if lineNum > len(lines) {
		return ""
	}

	// Show 2 lines before and after the error
	start := lineNum - 3
	if start < 0 {
		start = 0
	}
	end := lineNum + 2
	if end >= len(lines) {
		end = len(lines) - 1
	}

	var context strings.Builder
	for i := start; i <= end; i++ {
		marker := "  "
		if i == lineNum-1 { // Arrays are 0-based, line numbers are 1-based
			marker = "> "
		}
		context.WriteString(fmt.Sprintf("%s%d: %s\n", marker, i+1, lines[i]))
	}

	return context.String()
}

// validateTOMLStructure validates the TOML file structure and reports unknown fields
func (sp *SmogPing) validateTOMLStructure(filename string, metadata toml.MetaData, isDefault bool) error {
	validator := &ConfigValidator{}

	// Check for unknown/undefined fields
	undecoded := metadata.Undecoded()
	for _, key := range undecoded {
		keyStr := key.String()
		if isDefault {
			// Default config should not have unknown fields
			validator.AddError(&TOMLValidationError{
				File:    filename,
				Field:   keyStr,
				Message: "unknown configuration field",
			})
		} else {
			// Override config warns about unknown fields but doesn't fail
			validator.AddWarning(fmt.Sprintf("Unknown field '%s' in %s will be ignored", keyStr, filename))
		}
	}

	// Report warnings
	for _, warning := range validator.GetWarnings() {
		sp.verbosef("TOML Warning: %s", warning)
	}

	// Return errors if any
	if validator.HasErrors() {
		return fmt.Errorf("TOML structure validation failed: %v", validator.GetErrors()[0])
	}

	return nil
}

// validateConfigFields validates individual configuration field values
func (sp *SmogPing) validateConfigFields(filename string, config *Config, isDefault bool) error {
	validator := &ConfigValidator{}

	// Required fields validation (for default config)
	if isDefault {
		if config.InfluxURL == "" {
			validator.AddError(&TOMLValidationError{
				File: filename, Field: "influx_url", Value: config.InfluxURL,
				Message: "InfluxDB URL cannot be empty"})
		}
		if config.InfluxOrg == "" {
			validator.AddError(&TOMLValidationError{
				File: filename, Field: "influx_org", Value: config.InfluxOrg,
				Message: "InfluxDB organization cannot be empty"})
		}
		if config.InfluxBucket == "" {
			validator.AddError(&TOMLValidationError{
				File: filename, Field: "influx_bucket", Value: config.InfluxBucket,
				Message: "InfluxDB bucket cannot be empty"})
		}
	}

	// URL validation
	if config.InfluxURL != "" && !isValidURL(config.InfluxURL) {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "influx_url", Value: config.InfluxURL,
			Message: "invalid URL format"})
	}

	// Numeric range validations
	// All fields should be validated since we're loading a complete config file
	if config.InfluxBatchSize < 0 || config.InfluxBatchSize > 10000 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "influx_batch_size", Value: config.InfluxBatchSize,
			Message: "must be between 0 and 10000"})
	}

	if config.InfluxBatchTime < 0 || config.InfluxBatchTime > 3600 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "influx_batch_time", Value: config.InfluxBatchTime,
			Message: "must be between 0 and 3600 seconds"})
	}

	if config.DataPointPings < 1 || config.DataPointPings > 100 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "data_point_pings", Value: config.DataPointPings,
			Message: "must be between 1 and 100"})
	}

	if config.DataPointTime < 1 || config.DataPointTime > 86400 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "data_point_time", Value: config.DataPointTime,
			Message: "must be between 1 and 86400 seconds"})
	}

	if config.PingTimeout < 1 || config.PingTimeout > 60 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "ping_timeout", Value: config.PingTimeout,
			Message: "must be between 1 and 60 seconds"})
	}

	if config.DNSRefresh < 0 || config.DNSRefresh > 86400 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "dns_refresh", Value: config.DNSRefresh,
			Message: "must be between 0 and 86400 seconds"})
	}

	if config.AlarmRate < 0 || config.AlarmRate > 3600 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "alarm_rate", Value: config.AlarmRate,
			Message: "must be between 0 and 3600 seconds"})
	}

	if config.MaxConcurrentPings < 1 || config.MaxConcurrentPings > 1000 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "max_concurrent_pings", Value: config.MaxConcurrentPings,
			Message: "must be between 1 and 1000"})
	}

	// Validate ping_source (must be "default" or a valid IP address)
	if config.PingSource != "" && config.PingSource != "default" {
		if net.ParseIP(config.PingSource) == nil {
			validator.AddError(&TOMLValidationError{
				File: filename, Field: "ping_source", Value: config.PingSource,
				Message: "must be 'default' or a valid IP address"})
		}
	}

	// Logical validations
	if config.PingTimeout >= config.DataPointTime {
		validator.AddWarning(fmt.Sprintf("ping_timeout (%d) should be less than data_point_time (%d)",
			config.PingTimeout, config.DataPointTime))
	}

	// Report warnings
	for _, warning := range validator.GetWarnings() {
		sp.verbosef("Config Warning: %s", warning)
	}

	// Return first error if any
	if validator.HasErrors() {
		return validator.GetErrors()[0]
	}

	return nil
}

// isValidURL validates URL format
func isValidURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	// Basic URL validation
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return false
	}

	// Check for basic URL structure
	if len(urlStr) < 10 { // Minimum: http://a.b
		return false
	}

	return true
}

// loadTargets loads targets from specified targets file and included files
func (sp *SmogPing) loadTargets() error {
	sp.debugf("Loading target configuration from: %s", sp.targetsFile)

	// Load main targets file with validation
	if err := sp.loadAndValidateTargetsFile(sp.targetsFile, &sp.targets, true); err != nil {
		return fmt.Errorf("failed to load targets: %w", err)
	}
	sp.debugf("Loaded and validated %s", sp.targetsFile)

	// Load included files with validation
	for _, includeFile := range sp.targets.Include {
		// Resolve relative paths based on the main targets file directory
		resolvedIncludeFile := includeFile
		if !filepath.IsAbs(includeFile) {
			// Make relative paths relative to the main targets file directory
			targetsDir := filepath.Dir(sp.targetsFile)
			resolvedIncludeFile = filepath.Join(targetsDir, includeFile)
		}

		sp.debugf("Loading included file: %s (resolved from %s)", resolvedIncludeFile, includeFile)
		var includedTargets TargetsConfig

		if err := sp.loadAndValidateTargetsFile(resolvedIncludeFile, &includedTargets, false); err != nil {
			sp.syslogWarning("Failed to load included file %s: %v", resolvedIncludeFile, err)
			log.Printf("Warning: failed to load included file %s: %v", resolvedIncludeFile, err)
			continue
		}

		// Merge organizations
		for orgName, org := range includedTargets.Organizations {
			if existingOrg, exists := sp.targets.Organizations[orgName]; exists {
				// Merge hosts
				existingOrg.Hosts = append(existingOrg.Hosts, org.Hosts...)
				sp.targets.Organizations[orgName] = existingOrg
				sp.debugf("Merged %d hosts into existing organization %s", len(org.Hosts), orgName)
			} else {
				sp.targets.Organizations[orgName] = org
				sp.debugf("Added new organization %s with %d hosts", orgName, len(org.Hosts))
			}
		}
	}

	// Final validation of complete targets configuration
	if err := sp.validateCompleteTargets(); err != nil {
		return fmt.Errorf("complete targets validation failed: %w", err)
	}

	// Count total hosts
	totalHosts := 0
	for orgName, org := range sp.targets.Organizations {
		totalHosts += len(org.Hosts)
		if sp.verbose {
			log.Printf("Organization %s: %d hosts", orgName, len(org.Hosts))
		}
		if sp.debug {
			for _, host := range org.Hosts {
				sp.debugf("  %s (%s) - ping:%d loss:%d jitter:%d",
					host.Name, host.IP, host.AlarmPing, host.AlarmLoss, host.AlarmJitter)
			}
		}
	}

	// Calculate stagger rate for normal mode output
	hostsPerSecond := int(math.Ceil(float64(totalHosts) / float64(sp.config.DataPointTime)))

	// Show summary based on verbosity level
	if sp.verbose {
		log.Printf("Total hosts to monitor: %d", totalHosts)
		log.Printf("Starting %d hosts/second over %d seconds", hostsPerSecond, sp.config.DataPointTime)
	} else {
		log.Printf("Monitoring %d targets, starting %d hosts/second", totalHosts, hostsPerSecond)
	}

	// Log startup summary to syslog
	sp.syslogInfo("SmogPing started: monitoring %d targets, starting %d hosts/second over %d seconds",
		totalHosts, hostsPerSecond, sp.config.DataPointTime)

	return nil
}

// loadAndValidateTargetsFile loads a TOML targets file with comprehensive validation
func (sp *SmogPing) loadAndValidateTargetsFile(filename string, targets *TargetsConfig, isMain bool) error {
	// Check if file exists and is readable
	if _, err := os.Stat(filename); err != nil {
		if isMain {
			return fmt.Errorf("required targets file %s not found: %w", filename, err)
		}
		return fmt.Errorf("targets file %s not accessible: %w", filename, err)
	}

	// Check file permissions and size
	if err := sp.validateFileBasics(filename); err != nil {
		return fmt.Errorf("file validation failed for %s: %w", filename, err)
	}

	// Parse TOML with detailed error handling
	metadata, err := toml.DecodeFile(filename, targets)
	if err != nil {
		return sp.enhanceTOMLError(filename, err)
	}

	// Validate TOML structure
	if err := sp.validateTargetsTOMLStructure(filename, metadata, isMain); err != nil {
		return err
	}

	// Validate targets content
	if err := sp.validateTargetsContent(filename, targets, isMain); err != nil {
		return err
	}

	sp.debugf("Successfully loaded and validated %s", filename)
	return nil
}

// validateTargetsTOMLStructure validates the targets TOML file structure
func (sp *SmogPing) validateTargetsTOMLStructure(filename string, metadata toml.MetaData, isMain bool) error {
	validator := &ConfigValidator{}

	// Check for unknown fields
	undecoded := metadata.Undecoded()
	for _, key := range undecoded {
		keyStr := key.String()
		if isMain {
			// Main targets file should not have unknown fields
			validator.AddError(&TOMLValidationError{
				File:    filename,
				Field:   keyStr,
				Message: "unknown targets configuration field",
			})
		} else {
			// Include files warn about unknown fields
			validator.AddWarning(fmt.Sprintf("Unknown field '%s' in %s will be ignored", keyStr, filename))
		}
	}

	// Report warnings
	for _, warning := range validator.GetWarnings() {
		sp.verbosef("Targets Warning: %s", warning)
	}

	// Return errors if any
	if validator.HasErrors() {
		return fmt.Errorf("targets TOML structure validation failed: %v", validator.GetErrors()[0])
	}

	return nil
}

// validateTargetsContent validates the content of targets configuration
func (sp *SmogPing) validateTargetsContent(filename string, targets *TargetsConfig, isMain bool) error {
	validator := &ConfigValidator{}

	// Validate include files (only in main targets file)
	if isMain {
		for _, includeFile := range targets.Include {
			if includeFile == "" {
				validator.AddError(&TOMLValidationError{
					File: filename, Field: "include", Value: includeFile,
					Message: "include file path cannot be empty"})
				continue
			}

			// Resolve relative paths for validation
			resolvedIncludeFile := includeFile
			if !filepath.IsAbs(includeFile) {
				// Make relative paths relative to the main targets file directory
				targetsDir := filepath.Dir(filename)
				resolvedIncludeFile = filepath.Join(targetsDir, includeFile)
			}

			// Check if resolved include file path is reasonable
			if !isValidFilePath(resolvedIncludeFile) {
				validator.AddError(&TOMLValidationError{
					File: filename, Field: "include", Value: includeFile,
					Message: fmt.Sprintf("invalid include file path (resolved to: %s)", resolvedIncludeFile)})
			}
		}
	}

	// Validate organizations
	if len(targets.Organizations) == 0 && isMain {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "organizations", Value: len(targets.Organizations),
			Message: "at least one organization must be defined"})
	}

	// Validate each organization
	for orgName, org := range targets.Organizations {
		if err := sp.validateOrganization(filename, orgName, org, validator); err != nil {
			return err
		}
	}

	// Return first error if any
	if validator.HasErrors() {
		return validator.GetErrors()[0]
	}

	return nil
}

// validateOrganization validates an individual organization configuration
func (sp *SmogPing) validateOrganization(filename, orgName string, org Organization, validator *ConfigValidator) error {
	// Organization name validation
	if orgName == "" {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "organizations", Value: orgName,
			Message: "organization name cannot be empty"})
	}

	if len(orgName) > 100 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "organizations", Value: orgName,
			Message: "organization name too long (max 100 characters)"})
	}

	// Check for invalid characters in organization name
	if !isValidName(orgName) {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: "organizations", Value: orgName,
			Message: "organization name contains invalid characters"})
	}

	// Hosts validation
	if len(org.Hosts) == 0 {
		validator.AddWarning(fmt.Sprintf("Organization '%s' has no hosts defined", orgName))
		return nil
	}

	if len(org.Hosts) > 1000 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fmt.Sprintf("organizations.%s.hosts", orgName), Value: len(org.Hosts),
			Message: "too many hosts (max 1000 per organization)"})
	}

	// Validate each host
	hostNames := make(map[string]bool)
	hostIPs := make(map[string]bool)

	for i, host := range org.Hosts {
		if err := sp.validateHost(filename, orgName, i, host, validator); err != nil {
			return err
		}

		// Check for duplicate host names within organization
		if hostNames[host.Name] {
			validator.AddError(&TOMLValidationError{
				File: filename, Field: fmt.Sprintf("organizations.%s.hosts[%d].name", orgName, i),
				Value: host.Name, Message: "duplicate host name in organization"})
		}
		hostNames[host.Name] = true

		// Check for duplicate IPs within organization (warning only)
		if hostIPs[host.IP] {
			validator.AddWarning(fmt.Sprintf("Duplicate IP address '%s' for host '%s' in organization '%s'",
				host.IP, host.Name, orgName))
		}
		hostIPs[host.IP] = true
	}

	return nil
}

// validateHost validates an individual host configuration
func (sp *SmogPing) validateHost(filename, orgName string, index int, host Host, validator *ConfigValidator) error {
	fieldPrefix := fmt.Sprintf("organizations.%s.hosts[%d]", orgName, index)

	// Host name validation
	if host.Name == "" {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".name", Value: host.Name,
			Message: "host name cannot be empty"})
	}

	if len(host.Name) > 100 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".name", Value: host.Name,
			Message: "host name too long (max 100 characters)"})
	}

	if !isValidName(host.Name) {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".name", Value: host.Name,
			Message: "host name contains invalid characters"})
	}

	// IP/hostname validation
	if host.IP == "" {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".ip", Value: host.IP,
			Message: "IP address or hostname cannot be empty"})
	}

	if len(host.IP) > 253 { // Maximum DNS name length
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".ip", Value: host.IP,
			Message: "IP address or hostname too long (max 253 characters)"})
	}

	// Validate IP address or hostname format
	if !isValidIPOrHostname(host.IP) {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".ip", Value: host.IP,
			Message: "invalid IP address or hostname format"})
	}

	// Alarm threshold validation
	if host.AlarmPing < 0 || host.AlarmPing > 10000 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".alarmping", Value: host.AlarmPing,
			Message: "alarm ping threshold must be between 0 and 10000 ms"})
	}

	if host.AlarmLoss < 0 || host.AlarmLoss > 100 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".alarmloss", Value: host.AlarmLoss,
			Message: "alarm loss threshold must be between 0 and 100 percent"})
	}

	if host.AlarmJitter < 0 || host.AlarmJitter > 10000 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".alarmjitter", Value: host.AlarmJitter,
			Message: "alarm jitter threshold must be between 0 and 10000 ms"})
	}

	// Alarm receiver validation
	if host.AlarmReceiver != "" && len(host.AlarmReceiver) > 500 {
		validator.AddError(&TOMLValidationError{
			File: filename, Field: fieldPrefix + ".alarmreceiver", Value: host.AlarmReceiver,
			Message: "alarm receiver too long (max 500 characters)"})
	}

	// Ping source validation (per-host ping source, optional)
	if host.PingSource != "" && host.PingSource != "default" {
		if net.ParseIP(host.PingSource) == nil {
			validator.AddError(&TOMLValidationError{
				File: filename, Field: fieldPrefix + ".pingsource", Value: host.PingSource,
				Message: "must be 'default' or a valid IP address"})
		}
	}

	return nil
}

// validateCompleteTargets performs final validation on the complete targets configuration
func (sp *SmogPing) validateCompleteTargets() error {
	validator := &ConfigValidator{}

	// Check for empty configuration
	if len(sp.targets.Organizations) == 0 {
		validator.AddError(fmt.Errorf("no organizations defined"))
	}

	// Count total hosts and validate overall limits
	totalHosts := 0
	allHostNames := make(map[string]string) // hostname -> organization

	for orgName, org := range sp.targets.Organizations {
		totalHosts += len(org.Hosts)

		// Check for duplicate host names across organizations
		for _, host := range org.Hosts {
			if existingOrg, exists := allHostNames[host.Name]; exists {
				validator.AddWarning(fmt.Sprintf("Host name '%s' appears in both '%s' and '%s' organizations",
					host.Name, existingOrg, orgName))
			}
			allHostNames[host.Name] = orgName
		}
	}

	// Total hosts validation
	if totalHosts == 0 {
		validator.AddError(fmt.Errorf("no hosts defined across all organizations"))
	}

	if totalHosts > 10000 {
		validator.AddError(fmt.Errorf("too many total hosts (%d), maximum 10000", totalHosts))
	}

	// Performance validation
	hostsPerSecond := float64(totalHosts) / float64(sp.config.DataPointTime)

	if hostsPerSecond > 100 {
		validator.AddWarning(fmt.Sprintf("High ping rate: %.1f hosts/second may impact performance", hostsPerSecond))
	}

	// Report warnings
	for _, warning := range validator.GetWarnings() {
		sp.verbosef("Targets Warning: %s", warning)
	}

	// Return first error if any
	if validator.HasErrors() {
		return validator.GetErrors()[0]
	}

	sp.verbosef("Targets validation completed: %d organizations, %d total hosts",
		len(sp.targets.Organizations), totalHosts)

	return nil
}

// Helper validation functions
func isValidFilePath(path string) bool {
	if path == "" {
		return false
	}

	// Check for invalid characters
	invalidChars := []string{"\x00", "\n", "\r"}
	for _, char := range invalidChars {
		if strings.Contains(path, char) {
			return false
		}
	}

	// Check file extension
	ext := filepath.Ext(path)
	if ext != ".toml" && ext != ".tml" {
		return false
	}

	return true
}

func isValidName(name string) bool {
	if name == "" {
		return false
	}

	// Allow alphanumeric, underscore, hyphen, space, and dot
	validNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-\s\.]+$`)
	return validNameRegex.MatchString(name)
}

func isValidIPOrHostname(address string) bool {
	if address == "" {
		return false
	}

	// Try parsing as IP address first
	if net.ParseIP(address) != nil {
		return true
	}

	// Validate as hostname
	if len(address) > 253 {
		return false
	}

	// Basic hostname validation
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`)
	if !hostnameRegex.MatchString(address) {
		return false
	}

	// Check for valid domain parts
	parts := strings.Split(address, ".")
	for _, part := range parts {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
	}

	return true
}

// setupWorkerPool initializes the worker pool for ping operations
func (sp *SmogPing) setupWorkerPool() {
	// Set defaults if not configured
	if sp.config.MaxConcurrentPings == 0 {
		sp.config.MaxConcurrentPings = 50 // Default: max 50 concurrent pings
	}

	// Create worker pool
	sp.workerPool = &PingWorkerPool{
		jobQueue:   make(chan PingJob, sp.config.MaxConcurrentPings*2), // Buffer for jobs
		resultChan: make(chan *PingResult, sp.config.MaxConcurrentPings),
		quit:       make(chan bool),
	}

	// Start workers
	for i := 0; i < sp.config.MaxConcurrentPings; i++ {
		worker := &PingWorker{
			id:       i,
			app:      sp,
			jobQueue: sp.workerPool.jobQueue,
			quit:     make(chan bool),
		}
		sp.workerPool.workers = append(sp.workerPool.workers, worker)

		sp.workerPool.wg.Add(1)
		go worker.Start()
	}

	// Start result handler
	sp.wg.Add(1)
	go sp.handlePingResults()

	sp.verbosef("Worker pool configured: %d workers with job buffer size %d",
		sp.config.MaxConcurrentPings, sp.config.MaxConcurrentPings*2)
}

// stopWorkerPool gracefully stops the worker pool
func (sp *SmogPing) stopWorkerPool() {
	if sp.workerPool == nil {
		return
	}

	sp.verbosef("Stopping worker pool...")

	// Signal all workers to quit
	close(sp.workerPool.quit)
	for _, worker := range sp.workerPool.workers {
		close(worker.quit)
	}

	// Close job queue
	close(sp.workerPool.jobQueue)

	// Wait for all workers to finish
	sp.workerPool.wg.Wait()

	// Close result channel
	close(sp.workerPool.resultChan)

	sp.verbosef("Worker pool stopped")
}

// Start starts the worker to process ping jobs
func (pw *PingWorker) Start() {
	defer pw.app.workerPool.wg.Done()
	pw.app.debugf("Worker %d started", pw.id)

	for {
		select {
		case <-pw.quit:
			pw.app.debugf("Worker %d stopping", pw.id)
			return
		case <-pw.app.ctx.Done():
			pw.app.debugf("Worker %d stopping due to context cancellation", pw.id)
			return
		case job, ok := <-pw.jobQueue:
			if !ok {
				pw.app.debugf("Worker %d stopping due to closed job queue", pw.id)
				return
			}

			pw.app.debugf("Worker %d processing job: %s (%s)", pw.id, job.Host.Name, job.Host.IP)
			result := pw.app.pingHost(job.OrgName, job.Host)
			if result != nil {
				// Send result to result channel (non-blocking)
				select {
				case pw.app.workerPool.resultChan <- result:
					pw.app.debugf("Worker %d sent result for %s", pw.id, job.Host.Name)
				default:
					pw.app.debugf("Worker %d: result channel full, dropping result for %s", pw.id, job.Host.Name)
					// Return result to pool if channel is full
					pw.app.returnPingResultToPool(result)
				}
			}
		}
	}
}

// handlePingResults processes ping results from workers
func (sp *SmogPing) handlePingResults() {
	defer sp.wg.Done()
	sp.debugf("Result handler started")

	for {
		select {
		case <-sp.ctx.Done():
			sp.debugf("Result handler stopping due to context cancellation")
			return
		case result, ok := <-sp.workerPool.resultChan:
			if !ok {
				sp.debugf("Result handler stopping due to closed result channel")
				return
			}

			sp.debugf("Processing result for %s (%s)", result.Host.Name, result.Host.IP)

			// Write to InfluxDB
			sp.writeToInflux(*result)

			// Check alarms if enabled
			if !sp.noAlarm {
				sp.checkAlarms(*result)
			}

			// Return result object to pool
			sp.returnPingResultToPool(result)
		}
	}
}

// getPingResultFromPool gets a PingResult from the object pool
func (sp *SmogPing) getPingResultFromPool() *PingResult {
	result := pingResultPool.Get().(*PingResult)
	// Reset the result to clean state
	*result = PingResult{}
	return result
}

// returnPingResultToPool returns a PingResult to the object pool
func (sp *SmogPing) returnPingResultToPool(result *PingResult) {
	if result != nil {
		pingResultPool.Put(result)
	}
}

// getRTTSliceFromPool gets an RTT slice from the object pool
func (sp *SmogPing) getRTTSliceFromPool() []time.Duration {
	slice := rttSlicePool.Get().([]time.Duration)
	// Reset slice to zero length but keep capacity
	return slice[:0]
}

// returnRTTSliceToPool returns an RTT slice to the object pool
func (sp *SmogPing) returnRTTSliceToPool(slice []time.Duration) {
	if slice != nil && cap(slice) > 0 {
		rttSlicePool.Put(slice)
	}
}

// setupDNSResolver initializes the DNS resolver with caching
func (sp *SmogPing) setupDNSResolver() {
	sp.dnsResolver = &DNSResolver{
		cache: make(map[string]*DNSCache),
		resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: 5 * time.Second,
				}
				return d.DialContext(ctx, network, address)
			},
		},
	}

	// Set default DNS refresh interval if not configured
	if sp.config.DNSRefresh <= 0 {
		sp.config.DNSRefresh = 600 // Default: 10 minutes
	}

	sp.verbosef("DNS resolver configured with %d second refresh interval", sp.config.DNSRefresh)
}

// performDNSPreflightChecks resolves all DNS names in targets and validates them
// Failed DNS resolutions will be removed from targets rather than causing startup failure
func (sp *SmogPing) performDNSPreflightChecks() error {
	sp.verbosef("Performing DNS pre-flight checks...")

	sp.targetsMux.Lock()
	defer sp.targetsMux.Unlock()

	dnsHostCount := 0
	ipHostCount := 0
	errorCount := 0
	removedCount := 0

	for orgName, org := range sp.targets.Organizations {
		var validHosts []Host // Track hosts that pass DNS checks

		for _, host := range org.Hosts {
			// Check if IP field contains a DNS name or IP address
			if sp.isDNSName(host.IP) {
				host.IsDNSName = true
				dnsHostCount++
				sp.debugf("Host %s (%s) in %s: DNS name detected", host.Name, host.IP, orgName)

				// Resolve DNS name to IP
				resolvedIP, err := sp.resolveDNSName(host.IP)
				if err != nil {
					log.Printf("WARNING: Failed to resolve DNS name %s for host %s in %s: %v - removing from targets",
						host.IP, host.Name, orgName, err)
					errorCount++
					removedCount++
					continue // Skip this host - don't add to validHosts
				}

				host.ResolvedIP = resolvedIP
				host.LastDNSCheck = time.Now()

				sp.verbosef("Resolved %s -> %s for host %s in %s",
					host.IP, resolvedIP, host.Name, orgName)

				// Cache the DNS resolution
				sp.dnsResolver.cacheMux.Lock()
				sp.dnsResolver.cache[host.IP] = &DNSCache{
					Hostname:    host.IP,
					ResolvedIP:  resolvedIP,
					LastChecked: time.Now(),
					DNSChanges:  0,
				}
				sp.dnsResolver.cacheMux.Unlock()
			} else {
				host.IsDNSName = false
				host.ResolvedIP = host.IP // Use IP as-is
				ipHostCount++
				sp.debugf("Host %s (%s) in %s: IP address detected", host.Name, host.IP, orgName)
			}

			// Add valid host to the list
			validHosts = append(validHosts, host)
		}

		// Update organization with only valid hosts
		org.Hosts = validHosts
		sp.targets.Organizations[orgName] = org
	}

	sp.verbosef("DNS pre-flight checks completed: %d DNS names resolved, %d IP addresses, %d errors, %d hosts removed",
		dnsHostCount, ipHostCount, errorCount, removedCount)

	if removedCount > 0 {
		log.Printf("WARNING: %d hosts removed from targets due to DNS resolution failures", removedCount)
	}

	// Log DNS summary to syslog
	sp.syslogInfo("DNS pre-flight checks completed: %d DNS names resolved, %d IP addresses, %d errors, %d hosts removed",
		dnsHostCount, ipHostCount, errorCount, removedCount)

	return nil
}

// isDNSName checks if a string is a DNS name rather than an IP address
func (sp *SmogPing) isDNSName(address string) bool {
	// Try to parse as IP address
	if net.ParseIP(address) != nil {
		return false // It's a valid IP address
	}

	// Check if it looks like a domain name (contains dots and letters)
	if strings.Contains(address, ".") && strings.ContainsAny(address, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return true // Likely a DNS name
	}

	return false
}

// resolveDNSName resolves a DNS name to an IP address
func (sp *SmogPing) resolveDNSName(hostname string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ips, err := sp.dnsResolver.resolver.LookupHost(ctx, hostname)
	if err != nil {
		return "", fmt.Errorf("DNS resolution failed: %w", err)
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found for hostname %s", hostname)
	}

	// Return the first IPv4 address, or first address if no IPv4 found
	for _, ip := range ips {
		if parsedIP := net.ParseIP(ip); parsedIP != nil && parsedIP.To4() != nil {
			return ip, nil // IPv4 address
		}
	}

	// If no IPv4 found, return the first address
	return ips[0], nil
}

// startDNSRefreshMonitoring starts periodic DNS refresh checking
func (sp *SmogPing) startDNSRefreshMonitoring() {
	if sp.config.DNSRefresh <= 0 {
		sp.verbosef("DNS refresh monitoring disabled (dns_refresh = %d)", sp.config.DNSRefresh)
		return
	}

	ticker := time.NewTicker(time.Duration(sp.config.DNSRefresh) * time.Second)

	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-sp.ctx.Done():
				return
			case <-ticker.C:
				sp.performDNSRefreshCheck()
			}
		}
	}()

	sp.verbosef("Started DNS refresh monitoring with %d second intervals", sp.config.DNSRefresh)
}

// performDNSRefreshCheck checks all DNS names for IP changes
func (sp *SmogPing) performDNSRefreshCheck() {
	sp.verbosef("Performing DNS refresh check...")

	sp.targetsMux.Lock()
	defer sp.targetsMux.Unlock()

	checkedCount := 0
	changedCount := 0
	errorCount := 0

	for orgName, org := range sp.targets.Organizations {
		for i, host := range org.Hosts {
			if !host.IsDNSName {
				continue // Skip IP addresses
			}

			checkedCount++
			sp.debugf("Checking DNS for %s (%s) in %s", host.Name, host.IP, orgName)

			// Resolve current IP
			newIP, err := sp.resolveDNSName(host.IP)
			if err != nil {
				sp.debugf("DNS refresh failed for %s (%s) in %s: %v",
					host.Name, host.IP, orgName, err)
				errorCount++
				continue
			}

			oldIP := host.ResolvedIP
			if newIP != oldIP {
				log.Printf("DNS CHANGE: %s (%s) in %s changed from %s to %s",
					host.Name, host.IP, orgName, oldIP, newIP)

				// Update host with new IP
				host.ResolvedIP = newIP
				host.LastDNSCheck = time.Now()
				org.Hosts[i] = host
				changedCount++

				// Update DNS cache
				sp.dnsResolver.cacheMux.Lock()
				if cache, exists := sp.dnsResolver.cache[host.IP]; exists {
					cache.ResolvedIP = newIP
					cache.LastChecked = time.Now()
					cache.DNSChanges++
				} else {
					sp.dnsResolver.cache[host.IP] = &DNSCache{
						Hostname:    host.IP,
						ResolvedIP:  newIP,
						LastChecked: time.Now(),
						DNSChanges:  1,
					}
				}
				sp.dnsResolver.cacheMux.Unlock()

				// Log DNS change to syslog
				sp.syslogWarning("DNS CHANGE: %s (%s) in %s changed from %s to %s",
					host.Name, host.IP, orgName, oldIP, newIP)
			} else {
				host.LastDNSCheck = time.Now()
				org.Hosts[i] = host
				sp.debugf("DNS unchanged for %s (%s) in %s: %s",
					host.Name, host.IP, orgName, newIP)
			}
		}
		sp.targets.Organizations[orgName] = org
	}

	if changedCount > 0 || sp.verbose {
		log.Printf("DNS refresh check completed: %d checked, %d changed, %d errors",
			checkedCount, changedCount, errorCount)
	}

	if changedCount > 0 {
		sp.syslogInfo("DNS refresh completed: %d DNS names checked, %d changed, %d errors",
			checkedCount, changedCount, errorCount)
	}
}

// setupBatching initializes InfluxDB batching system
func (sp *SmogPing) setupBatching() {
	// Set defaults if not configured
	if sp.config.InfluxBatchSize <= 0 {
		sp.config.InfluxBatchSize = 100 // Default batch size
	}
	if sp.config.InfluxBatchTime <= 0 {
		sp.config.InfluxBatchTime = 10 // Default 10 seconds
	}

	// Initialize batching state
	sp.batchPoints = make([]*write.Point, 0, sp.config.InfluxBatchSize)
	sp.lastFlush = time.Now()

	// Start batch flush timer
	sp.wg.Add(1)
	go sp.batchFlushTimer()

	sp.verbosef("InfluxDB batching configured: BatchSize=%d, BatchTime=%ds",
		sp.config.InfluxBatchSize, sp.config.InfluxBatchTime)
}

// setupAlarms initializes the alarm system
func (sp *SmogPing) setupAlarms() {
	sp.lastAlarms = make(map[string]time.Time)

	sp.verbosef("Alarm system configured: AlarmRate=%ds", sp.config.AlarmRate)
}

// setupFileWatching initializes file system watching for configuration changes
func (sp *SmogPing) setupFileWatching() error {
	var err error
	sp.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Watch targets file and included files only
	filesToWatch := []string{sp.targetsFile}

	// Add included files to watch list
	for _, includeFile := range sp.targets.Include {
		filesToWatch = append(filesToWatch, includeFile)
	}

	for _, file := range filesToWatch {
		if _, err := os.Stat(file); err == nil {
			err := sp.watcher.Add(file)
			if err != nil {
				sp.verbosef("Warning: Failed to watch file %s: %v", file, err)
			} else {
				sp.verbosef("Watching file: %s", file)
			}
		}
	}

	// Initialize reload channel
	sp.reloadChan = make(chan bool, 1)

	// Start file watching goroutine
	sp.wg.Add(1)
	go sp.watchFiles()

	sp.verbosef("File watching configured for target changes")
	return nil
}

// watchFiles monitors configuration files for changes
func (sp *SmogPing) watchFiles() {
	defer sp.wg.Done()

	// Debounce timer to prevent multiple rapid reloads
	var debounceTimer *time.Timer
	debounceDelay := 2 * time.Second

	for {
		select {
		case <-sp.ctx.Done():
			return
		case event, ok := <-sp.watcher.Events:
			if !ok {
				return
			}

			sp.debugf("File event: %v", event)

			// Only process write and create events
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				sp.verbosef("Target file changed: %s", event.Name)

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDelay, func() {
					select {
					case sp.reloadChan <- true:
						sp.verbosef("Triggering target reload")
					default:
						sp.debugf("Reload already pending, skipping")
					}
				})
			}
		case err, ok := <-sp.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		case <-sp.reloadChan:
			sp.reloadConfiguration()
		}
	}
}

// reloadConfiguration reloads target files and updates targets
func (sp *SmogPing) reloadConfiguration() {
	sp.verbosef("Reloading targets...")

	// Create backup of current state
	sp.targetsMux.RLock()
	oldTargets := sp.targets
	sp.targetsMux.RUnlock()

	// Load new targets
	newTargets := TargetsConfig{Organizations: make(map[string]Organization)}

	// Reload targets
	if err := sp.reloadTargets(&newTargets); err != nil {
		log.Printf("Error reloading targets: %v - keeping current targets", err)
		return
	}

	// Apply changes with minimal disruption
	sp.applyTargetChanges(newTargets, oldTargets)
}

// reloadTargets reloads the targets configuration
func (sp *SmogPing) reloadTargets(newTargets *TargetsConfig) error {
	// Load main targets file with validation
	if err := sp.loadAndValidateTargetsFile(sp.targetsFile, newTargets, true); err != nil {
		return fmt.Errorf("failed to load targets: %w", err)
	}

	// Load included files with validation
	for _, includeFile := range newTargets.Include {
		var includedTargets TargetsConfig
		if err := sp.loadAndValidateTargetsFile(includeFile, &includedTargets, false); err != nil {
			sp.syslogWarning("Failed to reload included file %s: %v", includeFile, err)
			log.Printf("Warning: failed to reload included file %s: %v", includeFile, err)
			continue
		}

		// Merge organizations
		for orgName, org := range includedTargets.Organizations {
			if existingOrg, exists := newTargets.Organizations[orgName]; exists {
				existingOrg.Hosts = append(existingOrg.Hosts, org.Hosts...)
				newTargets.Organizations[orgName] = existingOrg
			} else {
				newTargets.Organizations[orgName] = org
			}
		}
	}

	// Final validation of reloaded targets
	// Temporarily store current targets for validation context
	originalTargets := sp.targets
	sp.targets = *newTargets

	err := sp.validateCompleteTargets()

	// Restore original targets if validation failed
	if err != nil {
		sp.targets = originalTargets
		return fmt.Errorf("reloaded targets validation failed: %w", err)
	}

	sp.debugf("Successfully reloaded and validated targets configuration")
	return nil
}

// applyTargetChanges applies new target configuration with minimal disruption
func (sp *SmogPing) applyTargetChanges(newTargets TargetsConfig, oldTargets TargetsConfig) {
	sp.verbosef("Applying target changes...")

	// Compare targets and identify changes
	added, removed, unchanged := sp.compareTargets(oldTargets, newTargets)

	// Update targets with write lock
	sp.targetsMux.Lock()
	sp.targets = newTargets
	sp.targetsMux.Unlock()

	// Report changes
	if len(added) > 0 || len(removed) > 0 {
		log.Printf("Target changes detected: %d added, %d removed, %d unchanged",
			len(added), len(removed), len(unchanged))

		if sp.verbose {
			if len(added) > 0 {
				log.Printf("Added targets:")
				for _, target := range added {
					log.Printf("  %s (%s) in %s", target.Host.Name, target.Host.IP, target.OrgName)
				}
			}
			if len(removed) > 0 {
				log.Printf("Removed targets:")
				for _, target := range removed {
					log.Printf("  %s (%s) in %s", target.Host.Name, target.Host.IP, target.OrgName)
				}
			}
		}

		// Update file watcher for new included files
		sp.updateWatchedFiles()

		// Log target changes to syslog
		totalTargets := 0
		for _, org := range newTargets.Organizations {
			totalTargets += len(org.Hosts)
		}
		hostsPerSecond := int(math.Ceil(float64(totalTargets) / float64(sp.config.DataPointTime)))
		sp.syslogInfo("Targets reloaded: monitoring %d targets, starting %d hosts/second over %d seconds",
			totalTargets, hostsPerSecond, sp.config.DataPointTime)
	} else {
		sp.verbosef("No target changes detected")
	}
}

// compareTargets compares old and new targets to identify changes
func (sp *SmogPing) compareTargets(oldTargets, newTargets TargetsConfig) (added, removed, unchanged []TargetInfo) {
	// Create maps for easier comparison
	oldMap := make(map[string]TargetInfo)
	newMap := make(map[string]TargetInfo)

	// Populate old targets map
	for orgName, org := range oldTargets.Organizations {
		for _, host := range org.Hosts {
			key := fmt.Sprintf("%s_%s_%s", orgName, host.Name, host.IP)
			oldMap[key] = TargetInfo{Host: host, OrgName: orgName}
		}
	}

	// Populate new targets map and identify added/unchanged
	for orgName, org := range newTargets.Organizations {
		for _, host := range org.Hosts {
			key := fmt.Sprintf("%s_%s_%s", orgName, host.Name, host.IP)
			targetInfo := TargetInfo{Host: host, OrgName: orgName}
			newMap[key] = targetInfo

			if _, exists := oldMap[key]; exists {
				unchanged = append(unchanged, targetInfo)
			} else {
				added = append(added, targetInfo)
			}
		}
	}

	// Identify removed targets
	for key, targetInfo := range oldMap {
		if _, exists := newMap[key]; !exists {
			removed = append(removed, targetInfo)
		}
	}

	return added, removed, unchanged
}

// updateWatchedFiles updates the file watcher for new included files
func (sp *SmogPing) updateWatchedFiles() {
	if sp.watcher == nil {
		return
	}

	// Get current watched files
	watchedFiles := make(map[string]bool)
	for _, watchedFile := range sp.watcher.WatchList() {
		watchedFiles[watchedFile] = true
	}

	// Add new included files to watch list
	for _, includeFile := range sp.targets.Include {
		if !watchedFiles[includeFile] {
			if _, err := os.Stat(includeFile); err == nil {
				err := sp.watcher.Add(includeFile)
				if err != nil {
					sp.verbosef("Warning: Failed to watch new included file %s: %v", includeFile, err)
				} else {
					sp.verbosef("Now watching new included file: %s", includeFile)
				}
			}
		}
	}
}

// batchFlushTimer periodically flushes batches based on time
func (sp *SmogPing) batchFlushTimer() {
	defer sp.wg.Done()

	ticker := time.NewTicker(time.Duration(sp.config.InfluxBatchTime) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sp.ctx.Done():
			// Final flush on shutdown
			sp.flushBatch("shutdown")
			return
		case <-ticker.C:
			sp.checkAndFlushBatch("timer")
		}
	}
}

// checkAndFlushBatch flushes batch if it has points and time has elapsed
func (sp *SmogPing) checkAndFlushBatch(reason string) {
	sp.batchMutex.Lock()
	defer sp.batchMutex.Unlock()

	if len(sp.batchPoints) > 0 && time.Since(sp.lastFlush) >= time.Duration(sp.config.InfluxBatchTime)*time.Second {
		sp.flushBatchUnsafe(reason)
	}
}

// flushBatch safely flushes the current batch
func (sp *SmogPing) flushBatch(reason string) {
	sp.batchMutex.Lock()
	defer sp.batchMutex.Unlock()
	sp.flushBatchUnsafe(reason)
}

// flushBatchUnsafe flushes batch without locking (must be called with lock held)
func (sp *SmogPing) flushBatchUnsafe(reason string) {
	if len(sp.batchPoints) == 0 {
		return
	}

	sp.debugf("Flushing batch of %d points (reason: %s)", len(sp.batchPoints), reason)

	// Write all points in batch
	for _, point := range sp.batchPoints {
		sp.influxWrite.WritePoint(point)
	}

	sp.verbosef("Flushed %d points to InfluxDB (reason: %s)", len(sp.batchPoints), reason)

	// Reset batch
	sp.batchPoints = sp.batchPoints[:0] // Keep capacity, reset length
	sp.lastFlush = time.Now()
}

// validateConfiguration performs sanity checks on the configuration and target count
func (sp *SmogPing) validateConfiguration() error {
	// Get current targets with read lock
	sp.targetsMux.RLock()
	currentTargets := sp.targets
	sp.targetsMux.RUnlock()

	// Count total hosts
	totalHosts := 0
	for _, org := range currentTargets.Organizations {
		totalHosts += len(org.Hosts)
	}

	// Calculate theoretical maximum targets that can be handled
	// Each ping sequence takes data_point_time seconds, and we can have max_concurrent_pings running
	// So maximum targets = max_concurrent_pings * data_point_time / data_point_time = max_concurrent_pings
	// But since ping sequences are staggered over data_point_time, the real limit is higher
	maxTargets := sp.config.MaxConcurrentPings * sp.config.DataPointTime

	if sp.verbose {
		log.Printf("Configuration validation:")
		log.Printf("  Total targets: %d", totalHosts)
		log.Printf("  Max concurrent pings: %d", sp.config.MaxConcurrentPings)
		log.Printf("  Data point time: %d seconds", sp.config.DataPointTime)
		log.Printf("  Theoretical maximum targets: %d", maxTargets)
	}

	// Check if we exceed the theoretical maximum
	if totalHosts > maxTargets {
		return fmt.Errorf("target count (%d) exceeds theoretical maximum (%d). "+
			"With %d max concurrent pings and %d second data point time, "+
			"you can monitor at most %d targets. "+
			"Consider increasing max_concurrent_pings or data_point_time",
			totalHosts, maxTargets, sp.config.MaxConcurrentPings,
			sp.config.DataPointTime, maxTargets)
	}

	// Warning if we're approaching the limit (80% or more)
	warningThreshold := int(float64(maxTargets) * 0.8)
	if totalHosts >= warningThreshold {
		log.Printf("WARNING: Target count (%d) is approaching the theoretical maximum (%d). "+
			"Consider monitoring system performance and potentially increasing max_concurrent_pings "+
			"if you plan to add more targets", totalHosts, maxTargets)
	}

	// Validate ping timing makes sense
	pingInterval := float64(sp.config.DataPointTime) / float64(sp.config.DataPointPings)
	if pingInterval < 1.0 {
		log.Printf("WARNING: Ping interval is very short (%.2f seconds). "+
			"With %d pings over %d seconds, pings will be sent every %.2f seconds. "+
			"Consider reducing data_point_pings or increasing data_point_time",
			pingInterval, sp.config.DataPointPings, sp.config.DataPointTime, pingInterval)
	}

	// Validate timeout vs ping interval
	if float64(sp.config.PingTimeout) > pingInterval {
		log.Printf("WARNING: Ping timeout (%d seconds) is longer than ping interval (%.2f seconds). "+
			"This may cause overlapping ping operations",
			sp.config.PingTimeout, pingInterval)
	}

	// Validate InfluxDB batch settings
	if sp.config.InfluxBatchSize <= 0 && sp.verbose {
		log.Printf("WARNING: InfluxDB batch size is %d, which may cause performance issues. "+
			"Consider setting influx_batch_size to a positive value (recommended: 100-1000)",
			sp.config.InfluxBatchSize)
	}

	if sp.config.InfluxBatchTime <= 0 && sp.verbose {
		log.Printf("WARNING: InfluxDB batch time is %d seconds, which may cause data loss. "+
			"Consider setting influx_batch_time to a positive value (recommended: 5-30 seconds)",
			sp.config.InfluxBatchTime)
	}

	// Calculate expected data points per interval
	dataPointsPerInterval := totalHosts
	dataPointsPerMinute := dataPointsPerInterval * (60 / sp.config.DataPointTime)
	if sp.verbose {
		log.Printf("  Expected data points: %d per %ds interval, ~%d per minute",
			dataPointsPerInterval, sp.config.DataPointTime, dataPointsPerMinute)
		log.Printf("Configuration validation completed successfully")
	}

	return nil
}

// setupInfluxDB initializes the InfluxDB client
func (sp *SmogPing) setupInfluxDB() error {
	client := influxdb2.NewClient(sp.config.InfluxURL, sp.config.InfluxToken)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := client.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to InfluxDB: %w", err)
	}

	if health.Status != "pass" {
		return fmt.Errorf("InfluxDB health check failed: %s", health.Status)
	}

	sp.influxWrite = client.WriteAPI(sp.config.InfluxOrg, sp.config.InfluxBucket)

	sp.verbosef("Connected to InfluxDB at %s", sp.config.InfluxURL)
	return nil
}

// startPingMonitoring starts individual ping schedules for each target
func (sp *SmogPing) startPingMonitoring() {
	// Calculate ping interval (time between individual pings)
	pingInterval := time.Duration(sp.config.DataPointTime) * time.Second / time.Duration(sp.config.DataPointPings)

	sp.verbosef("Starting ping monitoring: %d pings per %ds (interval: %v)",
		sp.config.DataPointPings, sp.config.DataPointTime, pingInterval)

	// Get current targets
	sp.targetsMux.RLock()
	currentTargets := sp.targets
	sp.targetsMux.RUnlock()

	// Start individual ping goroutines for each target with staggered starts
	hostIndex := 0
	totalHosts := 0
	for _, org := range currentTargets.Organizations {
		totalHosts += len(org.Hosts)
	}

	staggerDelay := pingInterval / time.Duration(totalHosts)
	if staggerDelay > 100*time.Millisecond {
		staggerDelay = 100 * time.Millisecond // Cap at 100ms
	}

	sp.verbosef("Starting %d individual ping schedules with %v stagger delay", totalHosts, staggerDelay)

	for orgName, org := range currentTargets.Organizations {
		for _, host := range org.Hosts {
			// Stagger the start times to avoid thundering herd
			startDelay := time.Duration(hostIndex) * staggerDelay
			hostIndex++

			sp.wg.Add(1)
			go func(orgName string, host Host, delay time.Duration) {
				defer sp.wg.Done()

				// Initial delay to stagger starts
				if delay > 0 {
					select {
					case <-sp.ctx.Done():
						return
					case <-time.After(delay):
					}
				}

				sp.runIndividualPingSchedule(orgName, host, pingInterval)
			}(orgName, host, startDelay)
		}
	}
}

// runIndividualPingSchedule runs a consistent ping schedule for a single target
func (sp *SmogPing) runIndividualPingSchedule(orgName string, host Host, pingInterval time.Duration) {
	// Initialize ping data collection for this host
	pingData := make([]time.Duration, 0, sp.config.DataPointPings)
	pingCount := 0
	dataPointStartTime := time.Now()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	sp.debugf("Started individual ping schedule for %s (%s): ping every %v", host.Name, host.IP, pingInterval)

	for {
		select {
		case <-sp.ctx.Done():
			return
		case <-ticker.C:
			// Send a single ping
			rtt, success := sp.sendSinglePing(host)

			if success {
				pingData = append(pingData, rtt)
				sp.debugf("Ping %d/%d for %s (%s): %v", len(pingData), sp.config.DataPointPings, host.Name, host.IP, rtt)
			} else {
				sp.debugf("Ping %d/%d for %s (%s): failed", pingCount+1, sp.config.DataPointPings, host.Name, host.IP)
			}

			pingCount++

			// Check if we have collected enough pings for a data point
			if pingCount >= sp.config.DataPointPings {
				// Calculate and store the data point
				sp.processDataPoint(orgName, host, pingData, dataPointStartTime)

				// Reset for next data point
				pingData = pingData[:0] // Reuse slice
				pingCount = 0
				dataPointStartTime = time.Now()
			}
		}
	}
}

// sendSinglePing sends a single ping to a host and returns the RTT and success status
func (sp *SmogPing) sendSinglePing(host Host) (time.Duration, bool) {
	// Use resolved IP if available, otherwise use original IP
	targetIP := host.ResolvedIP
	if targetIP == "" {
		targetIP = host.IP
	}

	// Create pinger
	pinger, err := probing.NewPinger(targetIP)
	if err != nil {
		sp.debugf("Failed to create pinger for %s (%s -> %s): %v", host.Name, host.IP, targetIP, err)
		return 0, false
	}

	// Set pinger options for single ping
	pinger.Count = 1
	pinger.Timeout = time.Duration(sp.config.PingTimeout) * time.Second
	pinger.SetPrivileged(false) // Use unprivileged mode

	// Set source IP if configured - check host-specific first, then global
	var sourceIP string
	if host.PingSource != "" && host.PingSource != "default" {
		sourceIP = host.PingSource
	} else if sp.config.PingSource != "" && sp.config.PingSource != "default" {
		sourceIP = sp.config.PingSource
	}

	if sourceIP != "" {
		pinger.Source = sourceIP
		sp.debugf("Using source IP %s for pinging %s", sourceIP, targetIP)
	}

	// Send ping
	err = pinger.Run()
	if err != nil {
		sp.debugf("Ping failed for %s (%s -> %s): %v", host.Name, host.IP, targetIP, err)
		return 0, false
	}

	// Check results
	stats := pinger.Statistics()
	if stats.PacketsRecv > 0 {
		return stats.AvgRtt, true
	}

	return 0, false
}

// processDataPoint calculates statistics and stores the data point
func (sp *SmogPing) processDataPoint(orgName string, host Host, rtts []time.Duration, startTime time.Time) {
	// Get result object from pool
	result := sp.getPingResultFromPool()
	defer sp.returnPingResultToPool(result)

	// Calculate statistics
	successfulPings := len(rtts)

	if successfulPings == 0 {
		// Complete packet loss
		result.Host = host
		result.AvgRTT = 0
		result.PacketLoss = 100.0
		result.Jitter = 0
		result.Timestamp = startTime
		result.OrgName = orgName

		sp.verbosef("Data point for %s (%s): 100%% packet loss", host.Name, host.IP)
	} else {
		// Calculate average RTT
		var totalRTT time.Duration
		for _, rtt := range rtts {
			totalRTT += rtt
		}
		avgRTT := totalRTT / time.Duration(successfulPings)

		// Calculate packet loss percentage
		packetLoss := float64(sp.config.DataPointPings-successfulPings) / float64(sp.config.DataPointPings) * 100.0

		// Calculate jitter (standard deviation of RTTs)
		var jitter time.Duration
		if successfulPings > 1 {
			variance := float64(0)
			avgRTTFloat := float64(avgRTT)
			for _, rtt := range rtts {
				diff := float64(rtt) - avgRTTFloat
				variance += diff * diff
			}
			variance /= float64(successfulPings)
			jitter = time.Duration(math.Sqrt(variance))
		}

		result.Host = host
		result.AvgRTT = avgRTT
		result.PacketLoss = packetLoss
		result.Jitter = jitter
		result.Timestamp = startTime
		result.OrgName = orgName

		sp.verbosef("Data point for %s (%s): avg=%v, loss=%.1f%%, jitter=%v",
			host.Name, host.IP, avgRTT, packetLoss, jitter)
	}

	// Store the result
	sp.storeResult(result)
}

// storeResult processes and stores a ping result
func (sp *SmogPing) storeResult(result *PingResult) {
	sp.debugf("Processing result for %s (%s)", result.Host.Name, result.Host.IP)

	// Write to InfluxDB
	sp.writeToInflux(*result)

	// Check alarms if enabled
	if !sp.noAlarm {
		sp.checkAlarms(*result)
	}
}

// pingAllTargets pings all configured targets using the worker pool with dynamic staggered starts
func (sp *SmogPing) pingAllTargets() {
	startTime := time.Now()

	// Get current targets with read lock
	sp.targetsMux.RLock()
	currentTargets := sp.targets
	sp.targetsMux.RUnlock()

	// Count total hosts
	totalHosts := 0
	for _, org := range currentTargets.Organizations {
		totalHosts += len(org.Hosts)
	}

	// Calculate dynamic stagger timing
	// hosts_per_second = ceil(total_hosts / data_point_time)
	hostsPerSecond := int(math.Ceil(float64(totalHosts) / float64(sp.config.DataPointTime)))

	sp.verbosef("Starting ping cycle for %d targets: %d hosts per second over %d seconds",
		totalHosts, hostsPerSecond, sp.config.DataPointTime)

	// Create a slice of all hosts with their org names for easier batching
	allJobs := make([]PingJob, 0, totalHosts)
	for orgName, org := range currentTargets.Organizations {
		for _, host := range org.Hosts {
			allJobs = append(allJobs, PingJob{
				OrgName: orgName,
				Host:    host,
			})
		}
	}

	// Start hosts in batches every second
	jobIndex := 0
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for jobIndex < len(allJobs) {
		// Calculate how many hosts to start in this batch
		remainingJobs := len(allJobs) - jobIndex
		batchSize := hostsPerSecond
		if remainingJobs < batchSize {
			batchSize = remainingJobs
		}

		sp.debugf("Submitting batch of %d jobs (jobs %d-%d of %d)",
			batchSize, jobIndex+1, jobIndex+batchSize, totalHosts)

		// Submit this batch of jobs to worker pool
		for i := 0; i < batchSize && jobIndex < len(allJobs); i++ {
			job := allJobs[jobIndex]
			jobIndex++

			sp.debugf("Submitting job for %s (%s)", job.Host.Name, job.Host.IP)

			// Submit job to worker pool (non-blocking)
			select {
			case sp.workerPool.jobQueue <- job:
				// Job submitted successfully
			default:
				// Job queue is full, log warning
				sp.debugf("Warning: Job queue full, skipping %s (%s)", job.Host.Name, job.Host.IP)
			}
		}

		// If there are more jobs to process, wait for next second
		if jobIndex < len(allJobs) {
			<-ticker.C
		}
	}

	elapsed := time.Since(startTime)
	sp.verbosef("Submitted all %d ping jobs in %v", totalHosts, elapsed)
}

// pingHost performs ping operations on a single host spread over the data point time interval
func (sp *SmogPing) pingHost(orgName string, host Host) *PingResult {
	// Use resolved IP if available, otherwise use original IP
	targetIP := host.ResolvedIP
	if targetIP == "" {
		targetIP = host.IP
	}

	// Calculate interval between pings
	pingInterval := time.Duration(sp.config.DataPointTime) * time.Second / time.Duration(sp.config.DataPointPings)

	sp.debugf("Pinging %s (%s -> %s): %d pings over %v (interval: %v)",
		host.Name, host.IP, targetIP, sp.config.DataPointPings,
		time.Duration(sp.config.DataPointTime)*time.Second, pingInterval)

	// Get RTT slice from pool for better memory efficiency
	rtts := sp.getRTTSliceFromPool()
	defer sp.returnRTTSliceToPool(rtts)

	successfulPings := 0
	startTime := time.Now()

	// Send pings at intervals with natural rate control via worker pool
	for i := 0; i < sp.config.DataPointPings; i++ {
		// Check if we should stop due to context cancellation
		select {
		case <-sp.ctx.Done():
			return nil
		default:
		}

		sp.debugf("Sending ping %d/%d to %s (%s -> %s)", i+1, sp.config.DataPointPings, host.Name, host.IP, targetIP)

		// Send single ping to resolved IP
		pinger, err := probing.NewPinger(targetIP)
		if err != nil {
			sp.debugf("Failed to create pinger for %s (%s -> %s) ping %d: %v", host.Name, host.IP, targetIP, i+1, err)
			continue
		}

		// Set pinger options for single ping
		pinger.Count = 1
		pinger.Timeout = time.Duration(sp.config.PingTimeout) * time.Second
		pinger.SetPrivileged(false) // Use unprivileged mode

		// Set source IP if configured - check host-specific first, then global
		var sourceIP string
		if host.PingSource != "" && host.PingSource != "default" {
			sourceIP = host.PingSource
		} else if sp.config.PingSource != "" && sp.config.PingSource != "default" {
			sourceIP = sp.config.PingSource
		}

		if sourceIP != "" {
			pinger.Source = sourceIP
		}

		pingStart := time.Now()
		err = pinger.Run()
		if err != nil {
			sp.debugf("Failed ping %d for %s (%s -> %s): %v", i+1, host.Name, host.IP, targetIP, err)
		} else {
			stats := pinger.Statistics()
			if stats.PacketsRecv > 0 {
				rtts = append(rtts, stats.AvgRtt)
				successfulPings++
				sp.debugf("Ping %d for %s (%s -> %s): %v", i+1, host.Name, host.IP, targetIP, stats.AvgRtt)
			} else {
				sp.debugf("Ping %d for %s (%s -> %s): no response", i+1, host.Name, host.IP, targetIP)
			}
		}

		// Wait for next ping interval (unless this is the last ping)
		if i < sp.config.DataPointPings-1 {
			elapsed := time.Since(pingStart)
			sleepTime := pingInterval - elapsed
			if sleepTime > 0 {
				time.Sleep(sleepTime)
			}
		}
	}

	// Get result object from pool
	result := sp.getPingResultFromPool()

	// Calculate aggregated results
	if len(rtts) == 0 {
		result.Host = host
		result.AvgRTT = 0
		result.PacketLoss = 100.0
		result.Jitter = 0
		result.Timestamp = startTime
		result.OrgName = orgName

		sp.verbosef("Ping summary for %s (%s -> %s): 100%% packet loss", host.Name, host.IP, targetIP)
		return result
	}

	// Calculate average RTT
	var totalRTT time.Duration
	for _, rtt := range rtts {
		totalRTT += rtt
	}
	avgRTT := totalRTT / time.Duration(len(rtts))

	// Calculate packet loss percentage
	packetLoss := float64(sp.config.DataPointPings-successfulPings) / float64(sp.config.DataPointPings) * 100.0

	// Calculate jitter (standard deviation of RTTs)
	var jitter time.Duration
	if len(rtts) > 1 {
		variance := float64(0)
		for _, rtt := range rtts {
			diff := float64(rtt - avgRTT)
			variance += diff * diff
		}
		variance /= float64(len(rtts) - 1)
		jitter = time.Duration(math.Sqrt(variance))
	}

	// Fill result object
	result.Host = host
	result.AvgRTT = avgRTT
	result.PacketLoss = packetLoss
	result.Jitter = jitter
	result.Timestamp = startTime
	result.OrgName = orgName

	sp.verbosef("Ping summary for %s (%s -> %s): avg=%.1fms, loss=%.1f%%, jitter=%.1fms",
		host.Name, host.IP, targetIP,
		float64(avgRTT.Nanoseconds())/1e6,
		packetLoss,
		float64(jitter.Nanoseconds())/1e6)

	if sp.debug {
		sp.debugf("Data point calculated for %s (%s -> %s):", host.Name, host.IP, targetIP)
		sp.debugf("  RTTs collected: %v", rtts)
		sp.debugf("  Successful pings: %d/%d", successfulPings, sp.config.DataPointPings)
		sp.debugf("  Average RTT: %v (%.1fms)", avgRTT, float64(avgRTT.Nanoseconds())/1e6)
		sp.debugf("  Packet loss: %.1f%%", packetLoss)
		sp.debugf("  Jitter: %v (%.1fms)", jitter, float64(jitter.Nanoseconds())/1e6)
		sp.debugf("  Timestamp: %s", startTime.Format(time.RFC3339))
	}

	return result
}

// writeToInflux writes ping results to InfluxDB with batching
func (sp *SmogPing) writeToInflux(result PingResult) {
	// Use resolved IP if available for the actual ping target
	targetIP := result.Host.ResolvedIP
	if targetIP == "" {
		targetIP = result.Host.IP
	}

	// Determine effective source IP for tags
	var effectiveSource string
	if result.Host.PingSource != "" && result.Host.PingSource != "default" {
		effectiveSource = result.Host.PingSource
	} else if sp.config.PingSource != "" && sp.config.PingSource != "default" {
		effectiveSource = sp.config.PingSource
	} else {
		effectiveSource = "default"
	}

	tags := map[string]string{
		"host":         result.Host.Name,
		"ip":           result.Host.IP, // Original IP/hostname
		"organization": result.OrgName,
		"source":       effectiveSource,
	}

	// Add resolved IP as a tag if different from original
	if result.Host.IsDNSName && targetIP != result.Host.IP {
		tags["resolved_ip"] = targetIP
		tags["is_dns_name"] = "true"
	} else {
		tags["is_dns_name"] = "false"
	}

	point := influxdb2.NewPoint("ping", tags,
		map[string]interface{}{
			"rtt_avg":     float64(result.AvgRTT.Nanoseconds()) / 1e6, // Convert to milliseconds
			"packet_loss": result.PacketLoss,
			"jitter":      float64(result.Jitter.Nanoseconds()) / 1e6, // Convert to milliseconds
		},
		result.Timestamp)

	sp.debugf("Created InfluxDB point for %s (%s -> %s): rtt=%.1fms, loss=%.1f%%, jitter=%.1fms",
		result.Host.Name, result.Host.IP, targetIP,
		float64(result.AvgRTT.Nanoseconds())/1e6,
		result.PacketLoss,
		float64(result.Jitter.Nanoseconds())/1e6)

	// Add to batch
	sp.batchMutex.Lock()
	sp.batchPoints = append(sp.batchPoints, point)
	batchSize := len(sp.batchPoints)
	sp.batchMutex.Unlock()

	sp.debugf("Added point to batch (current size: %d/%d)", batchSize, sp.config.InfluxBatchSize)

	// Check if we need to flush due to size
	if batchSize >= sp.config.InfluxBatchSize {
		sp.flushBatch("size")
	}
}

// checkAlarms evaluates ping results against alarm thresholds
func (sp *SmogPing) checkAlarms(result PingResult) {
	host := result.Host

	// Skip alarm checking if no alarm thresholds are configured
	if host.AlarmPing == 0 && host.AlarmLoss == 0 && host.AlarmJitter == 0 {
		sp.debugf("No alarm thresholds configured for %s (%s), skipping alarm check", host.Name, host.IP)
		return
	}

	// Skip alarm checking if no alarm receiver is configured
	alarmReceiver := host.AlarmReceiver
	if alarmReceiver == "" {
		alarmReceiver = sp.config.AlarmReceiver
	}
	if alarmReceiver == "" || strings.ToLower(alarmReceiver) == "none" {
		sp.debugf("No alarm receiver configured for %s (%s), skipping alarm check", host.Name, host.IP)
		return
	}

	sp.debugf("Checking alarms for %s (%s): ping_threshold=%d, loss_threshold=%d, jitter_threshold=%d",
		host.Name, host.IP, host.AlarmPing, host.AlarmLoss, host.AlarmJitter)

	// Check if we're within the alarm rate limit
	hostKey := fmt.Sprintf("%s_%s", result.OrgName, host.Name)
	sp.alarmMutex.RLock()
	lastAlarm, exists := sp.lastAlarms[hostKey]
	sp.alarmMutex.RUnlock()

	if exists && time.Since(lastAlarm) < time.Duration(sp.config.AlarmRate)*time.Second {
		// Still within alarm rate limit, skip
		sp.debugf("Alarm rate limit active for %s (%s), last alarm: %v ago",
			host.Name, host.IP, time.Since(lastAlarm))
		return
	}

	var alarmReasons []string

	// Check ping time alarm (alarmping is in milliseconds)
	if host.AlarmPing > 0 {
		avgRTTMs := float64(result.AvgRTT.Nanoseconds()) / 1e6
		if avgRTTMs > float64(host.AlarmPing) {
			alarmReasons = append(alarmReasons, fmt.Sprintf("ping_time=%.1fms>%dms", avgRTTMs, host.AlarmPing))
			sp.debugf("Ping time alarm triggered for %s (%s): %.1fms > %dms",
				host.Name, host.IP, avgRTTMs, host.AlarmPing)
		}
	}

	// Check packet loss alarm (alarmloss is in percentage)
	if host.AlarmLoss > 0 {
		if result.PacketLoss > float64(host.AlarmLoss) {
			alarmReasons = append(alarmReasons, fmt.Sprintf("packet_loss=%.1f%%>%d%%", result.PacketLoss, host.AlarmLoss))
			sp.debugf("Packet loss alarm triggered for %s (%s): %.1f%% > %d%%",
				host.Name, host.IP, result.PacketLoss, host.AlarmLoss)
		}
	}

	// Check jitter alarm (alarmjitter is in milliseconds)
	if host.AlarmJitter > 0 {
		jitterMs := float64(result.Jitter.Nanoseconds()) / 1e6
		if jitterMs > float64(host.AlarmJitter) {
			alarmReasons = append(alarmReasons, fmt.Sprintf("jitter=%.1fms>%dms", jitterMs, host.AlarmJitter))
			sp.debugf("Jitter alarm triggered for %s (%s): %.1fms > %dms",
				host.Name, host.IP, jitterMs, host.AlarmJitter)
		}
	}

	// If any alarms triggered, execute alarm receiver
	if len(alarmReasons) > 0 {
		sp.triggerAlarm(result, alarmReasons)

		// Update last alarm time
		sp.alarmMutex.Lock()
		sp.lastAlarms[hostKey] = time.Now()
		sp.alarmMutex.Unlock()
	} else {
		sp.debugf("No alarm thresholds exceeded for %s (%s)", host.Name, host.IP)
	}
}

// triggerAlarm executes the alarm receiver script
func (sp *SmogPing) triggerAlarm(result PingResult, reasons []string) {
	host := result.Host

	// Determine which alarm receiver to use
	alarmReceiver := host.AlarmReceiver
	if alarmReceiver == "" {
		alarmReceiver = sp.config.AlarmReceiver
	}

	if alarmReceiver == "" {
		log.Printf("ALARM: %s (%s) - %v - No alarm receiver configured",
			host.Name, host.IP, reasons)
		// Log alarm to syslog (unless disabled)
		if !sp.noLog {
			sp.syslogWarning("ALARM: %s (%s) in %s - %s - No alarm receiver configured",
				host.Name, host.IP, result.OrgName, strings.Join(reasons, ", "))
		}
		return
	}

	// Prepare alarm data as environment variables and command line arguments
	reasonsStr := fmt.Sprintf("[%s]", strings.Join(reasons, ", "))

	log.Printf("ALARM: %s (%s) - %s - Executing: %s",
		host.Name, host.IP, reasonsStr, alarmReceiver)

	// Log alarm to syslog (unless disabled)
	if !sp.noLog {
		sp.syslogWarning("ALARM: %s (%s) in %s - %s - RTT=%.1fms LOSS=%.1f%% JITTER=%.1fms",
			host.Name, host.IP, result.OrgName, strings.Join(reasons, ", "),
			float64(result.AvgRTT.Nanoseconds())/1e6, result.PacketLoss,
			float64(result.Jitter.Nanoseconds())/1e6)
	}

	// Execute alarm receiver in background
	go sp.executeAlarmReceiver(alarmReceiver, result, reasons)
}

// executeAlarmReceiver runs the alarm receiver script with alarm data
func (sp *SmogPing) executeAlarmReceiver(receiverPath string, result PingResult, reasons []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	host := result.Host

	sp.debugf("Executing alarm receiver: %s for %s (%s)", receiverPath, host.Name, host.IP)

	// Prepare command arguments
	args := []string{
		receiverPath,
		host.Name,      // $1: Host name
		host.IP,        // $2: Host IP
		result.OrgName, // $3: Organization
		fmt.Sprintf("%.1f", float64(result.AvgRTT.Nanoseconds())/1e6), // $4: RTT in ms
		fmt.Sprintf("%.1f", result.PacketLoss),                        // $5: Packet loss %
		fmt.Sprintf("%.1f", float64(result.Jitter.Nanoseconds())/1e6), // $6: Jitter in ms
		strings.Join(reasons, ","),                                    // $7: Alarm reasons
		result.Timestamp.Format(time.RFC3339),                         // $8: Timestamp
	}

	sp.debugf("Alarm receiver args: %v", args[1:]) // Skip the script path

	// Create command
	cmd := exec.CommandContext(ctx, "/bin/bash", args...)

	// Set environment variables
	env := []string{
		fmt.Sprintf("SMOGPING_HOST=%s", host.Name),
		fmt.Sprintf("SMOGPING_IP=%s", host.IP),
		fmt.Sprintf("SMOGPING_ORG=%s", result.OrgName),
		fmt.Sprintf("SMOGPING_RTT=%.1f", float64(result.AvgRTT.Nanoseconds())/1e6),
		fmt.Sprintf("SMOGPING_LOSS=%.1f", result.PacketLoss),
		fmt.Sprintf("SMOGPING_JITTER=%.1f", float64(result.Jitter.Nanoseconds())/1e6),
		fmt.Sprintf("SMOGPING_REASONS=%s", strings.Join(reasons, ",")),
		fmt.Sprintf("SMOGPING_TIMESTAMP=%s", result.Timestamp.Format(time.RFC3339)),
		fmt.Sprintf("SMOGPING_ALARM_PING=%d", host.AlarmPing),
		fmt.Sprintf("SMOGPING_ALARM_LOSS=%d", host.AlarmLoss),
		fmt.Sprintf("SMOGPING_ALARM_JITTER=%d", host.AlarmJitter),
	}

	cmd.Env = append(os.Environ(), env...)

	if sp.debug {
		sp.debugf("Alarm receiver environment variables:")
		for _, envVar := range env {
			sp.debugf("  %s", envVar)
		}
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ERROR: Alarm receiver failed for %s (%s): %v - Output: %s",
			host.Name, host.IP, err, string(output))
	} else {
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			log.Printf("Alarm receiver completed for %s (%s) - Output: %s",
				host.Name, host.IP, outputStr)
		} else {
			sp.verbosef("Alarm receiver completed for %s (%s) - No output", host.Name, host.IP)
		}
	}
}
