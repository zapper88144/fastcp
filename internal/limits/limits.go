package limits

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fastcp/fastcp/internal/models"
)

// Manager handles resource limits for users
type Manager struct {
	logger     *slog.Logger
	cgroupPath string
}

// NewManager creates a new limits manager
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger:     logger,
		cgroupPath: "/sys/fs/cgroup",
	}
}

// ApplyLimits applies resource limits to a user
func (m *Manager) ApplyLimits(limits *models.UserLimits) error {
	if runtime.GOOS != "linux" {
		m.logger.Debug("resource limits only supported on Linux")
		return nil
	}

	var errors []string

	// Apply cgroup limits (CPU, RAM, processes)
	if err := m.applyCgroupLimits(limits); err != nil {
		errors = append(errors, fmt.Sprintf("cgroup: %v", err))
	}

	// Apply disk quota
	if err := m.applyDiskQuota(limits); err != nil {
		errors = append(errors, fmt.Sprintf("quota: %v", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to apply some limits: %s", strings.Join(errors, "; "))
	}

	m.logger.Info("applied resource limits",
		"user", limits.Username,
		"disk_mb", limits.MaxDiskMB,
		"ram_mb", limits.MaxRAMMB,
		"cpu_percent", limits.MaxCPUPercent,
		"processes", limits.MaxProcesses,
	)

	return nil
}

// applyCgroupLimits creates/updates cgroup for user
func (m *Manager) applyCgroupLimits(limits *models.UserLimits) error {
	// Check if cgroup v2 is available
	if !m.isCgroupV2() {
		m.logger.Warn("cgroup v2 not available, resource limits may not work")
		return nil
	}

	cgroupName := fmt.Sprintf("fastcp-%s", limits.Username)
	cgroupDir := filepath.Join(m.cgroupPath, cgroupName)

	// Create cgroup directory if it doesn't exist
	if err := os.MkdirAll(cgroupDir, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup: %w", err)
	}

	// Enable controllers
	controllers := "+cpu +memory +pids"
	if err := os.WriteFile(filepath.Join(m.cgroupPath, "cgroup.subtree_control"), []byte(controllers), 0644); err != nil {
		m.logger.Warn("failed to enable cgroup controllers", "error", err)
	}

	// Apply memory limit
	if limits.MaxRAMMB > 0 {
		memBytes := limits.MaxRAMMB * 1024 * 1024
		memFile := filepath.Join(cgroupDir, "memory.max")
		if err := os.WriteFile(memFile, []byte(strconv.FormatInt(memBytes, 10)), 0644); err != nil {
			m.logger.Warn("failed to set memory limit", "error", err)
		}
	}

	// Apply CPU limit
	if limits.MaxCPUPercent > 0 {
		// cpu.max format: "$MAX $PERIOD" (microseconds)
		// 100000 = 100ms period, so for 50% = "50000 100000"
		period := 100000
		quota := (limits.MaxCPUPercent * period) / 100
		cpuMax := fmt.Sprintf("%d %d", quota, period)
		cpuFile := filepath.Join(cgroupDir, "cpu.max")
		if err := os.WriteFile(cpuFile, []byte(cpuMax), 0644); err != nil {
			m.logger.Warn("failed to set CPU limit", "error", err)
		}
	}

	// Apply process limit
	if limits.MaxProcesses > 0 {
		pidsFile := filepath.Join(cgroupDir, "pids.max")
		if err := os.WriteFile(pidsFile, []byte(strconv.Itoa(limits.MaxProcesses)), 0644); err != nil {
			m.logger.Warn("failed to set process limit", "error", err)
		}
	}

	return nil
}

// applyDiskQuota sets disk quota for user
func (m *Manager) applyDiskQuota(limits *models.UserLimits) error {
	if limits.MaxDiskMB == 0 {
		return nil
	}

	// Check if quota is available
	if _, err := exec.LookPath("setquota"); err != nil {
		m.logger.Warn("setquota not available, disk quotas disabled")
		return nil
	}

	// Get user's primary group
	u, err := m.lookupUser(limits.Username)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Set quota on /var/www (or wherever sites are stored)
	// Format: setquota -u username soft_block hard_block soft_inode hard_inode filesystem
	// Block size is typically 1KB
	softLimit := limits.MaxDiskMB * 1024        // 90% of hard limit
	hardLimit := limits.MaxDiskMB * 1024
	softInodes := int64(100000)
	hardInodes := int64(150000)

	// Find the filesystem for /var/www
	filesystem, err := m.getFilesystem("/var/www")
	if err != nil {
		m.logger.Warn("could not determine filesystem for /var/www", "error", err)
		return nil
	}

	cmd := exec.Command("setquota", "-u", u.Username,
		strconv.FormatInt(softLimit, 10),
		strconv.FormatInt(hardLimit, 10),
		strconv.FormatInt(softInodes, 10),
		strconv.FormatInt(hardInodes, 10),
		filesystem,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		m.logger.Warn("failed to set disk quota", "error", err, "output", string(output))
		// Don't return error - quotas might not be enabled on filesystem
		return nil
	}

	return nil
}

// GetUsage returns current resource usage for a user
func (m *Manager) GetUsage(username string) (*ResourceUsage, error) {
	usage := &ResourceUsage{
		Username: username,
	}

	if runtime.GOOS != "linux" {
		return usage, nil
	}

	// Get cgroup usage
	cgroupDir := filepath.Join(m.cgroupPath, fmt.Sprintf("fastcp-%s", username))

	// Memory usage
	if data, err := os.ReadFile(filepath.Join(cgroupDir, "memory.current")); err == nil {
		if bytes, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			usage.RAMUsedMB = bytes / (1024 * 1024)
		}
	}

	// CPU usage (this is cumulative, would need to calculate rate)
	if data, err := os.ReadFile(filepath.Join(cgroupDir, "cpu.stat")); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "usage_usec") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					if usec, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						usage.CPUUsageMicros = usec
					}
				}
			}
		}
	}

	// Process count
	if data, err := os.ReadFile(filepath.Join(cgroupDir, "pids.current")); err == nil {
		if count, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			usage.ProcessCount = count
		}
	}

	// Disk usage
	diskUsage, err := m.getDiskUsage(username)
	if err == nil {
		usage.DiskUsedMB = diskUsage
	}

	return usage, nil
}

// ResourceUsage represents current resource usage
type ResourceUsage struct {
	Username       string `json:"username"`
	RAMUsedMB      int64  `json:"ram_used_mb"`
	CPUUsageMicros int64  `json:"cpu_usage_micros"`
	DiskUsedMB     int64  `json:"disk_used_mb"`
	ProcessCount   int    `json:"process_count"`
}

// AddProcessToCgroup adds a process to user's cgroup
func (m *Manager) AddProcessToCgroup(username string, pid int) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	cgroupDir := filepath.Join(m.cgroupPath, fmt.Sprintf("fastcp-%s", username))
	procsFile := filepath.Join(cgroupDir, "cgroup.procs")

	return os.WriteFile(procsFile, []byte(strconv.Itoa(pid)), 0644)
}

// RemoveLimits removes cgroup for a user
func (m *Manager) RemoveLimits(username string) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	cgroupDir := filepath.Join(m.cgroupPath, fmt.Sprintf("fastcp-%s", username))

	// Move all processes out first
	procsFile := filepath.Join(cgroupDir, "cgroup.procs")
	if data, err := os.ReadFile(procsFile); err == nil {
		pids := strings.Fields(string(data))
		for _, pid := range pids {
			// Move to root cgroup
			_ = os.WriteFile(filepath.Join(m.cgroupPath, "cgroup.procs"), []byte(pid), 0644)
		}
	}

	return os.RemoveAll(cgroupDir)
}

// Helper methods

func (m *Manager) isCgroupV2() bool {
	_, err := os.Stat(filepath.Join(m.cgroupPath, "cgroup.controllers"))
	return err == nil
}

type userInfo struct {
	Username string
	UID      int
	GID      int
}

func (m *Manager) lookupUser(username string) (*userInfo, error) {
	cmd := exec.Command("id", "-u", username)
	uidOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	uid, _ := strconv.Atoi(strings.TrimSpace(string(uidOutput)))

	cmd = exec.Command("id", "-g", username)
	gidOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	gid, _ := strconv.Atoi(strings.TrimSpace(string(gidOutput)))

	return &userInfo{
		Username: username,
		UID:      uid,
		GID:      gid,
	}, nil
}

func (m *Manager) getFilesystem(path string) (string, error) {
	cmd := exec.Command("df", "-P", path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected df output")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 1 {
		return "", fmt.Errorf("unexpected df output")
	}

	return fields[0], nil
}

func (m *Manager) getDiskUsage(username string) (int64, error) {
	// Check quota usage first
	cmd := exec.Command("quota", "-u", username)
	output, err := cmd.Output()
	if err == nil {
		// Parse quota output
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "/") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					// Used blocks (in KB)
					if used, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						return used / 1024, nil // Convert to MB
					}
				}
			}
		}
	}

	// Fallback to du
	userDir := filepath.Join("/var/www", username)
	cmd = exec.Command("du", "-sm", userDir)
	output, err = cmd.Output()
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(output))
	if len(fields) >= 1 {
		mb, _ := strconv.ParseInt(fields[0], 10, 64)
		return mb, nil
	}

	return 0, nil
}

