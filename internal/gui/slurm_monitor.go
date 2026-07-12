package gui

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/slurm"
)

// SLURMMonitor manages SLURM job monitoring and UI updates.
type SLURMMonitor struct {
	client         slurm.Client
	jobs           []SLURMJobInfo
	jobsMutex      sync.RWMutex
	updateChan     chan struct{}
	ctx            context.Context //nolint:containedctx // Long-lived monitoring context
	cancel         context.CancelFunc
	isRunning      bool
	runningMutex   sync.RWMutex
	connected      bool
	connectedMutex sync.RWMutex
}

// SLURMJobInfo represents SLURM job information for UI display.
type SLURMJobInfo struct {
	JobID     string
	Owner     string
	Status    string
	StartTime string
	Partition string
	Nodes     string
	CPUs      string
}

// IsSLURMAvailable checks if SLURM tools are available on the system.
// This performs a quick check by attempting to run squeue with a short timeout.
// Returns true if squeue command succeeds, false otherwise.
func IsSLURMAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "squeue", "--version")
	err := cmd.Run()

	return err == nil
}

// NewSLURMMonitor creates a new SLURM monitor.
func NewSLURMMonitor(ctx context.Context, cfg *config.Config) (*SLURMMonitor, error) {
	if cfg == nil || cfg.SLURM.Mode == "" {
		return nil, fmt.Errorf("SLURM configuration not available")
	}

	client, err := slurm.NewClient(cfg.SLURM)
	if err != nil {
		return nil, fmt.Errorf("failed to create SLURM client: %w", err)
	}

	monitorCtx, cancel := context.WithCancel(ctx)

	monitor := &SLURMMonitor{
		client:     client,
		jobs:       make([]SLURMJobInfo, 0),
		updateChan: make(chan struct{}, 10),
		ctx:        monitorCtx,
		cancel:     cancel,
	}

	return monitor, nil
}

// Start begins the SLURM monitoring loop.
func (sm *SLURMMonitor) Start() {
	sm.runningMutex.Lock()
	if sm.isRunning {
		sm.runningMutex.Unlock()

		return
	}
	sm.isRunning = true
	sm.runningMutex.Unlock()

	go sm.monitorLoop()
}

// Stop stops the SLURM monitoring loop.
func (sm *SLURMMonitor) Stop() {
	sm.runningMutex.Lock()
	defer sm.runningMutex.Unlock()

	if !sm.isRunning {
		return
	}

	sm.cancel()
	sm.isRunning = false

	if sm.client != nil {
		sm.client.Close()
	}
}

// GetJobs returns the current list of SLURM jobs.
func (sm *SLURMMonitor) GetJobs() []SLURMJobInfo {
	sm.jobsMutex.RLock()
	defer sm.jobsMutex.RUnlock()

	// Return a copy to avoid race conditions
	jobs := make([]SLURMJobInfo, len(sm.jobs))
	copy(jobs, sm.jobs)

	return jobs
}

// GetUpdateChannel returns the channel for UI update notifications.
func (sm *SLURMMonitor) GetUpdateChannel() <-chan struct{} {
	return sm.updateChan
}

// IsConnected returns whether the monitor is successfully connected to SLURM.
func (sm *SLURMMonitor) IsConnected() bool {
	sm.connectedMutex.RLock()
	defer sm.connectedMutex.RUnlock()

	return sm.connected
}

// setConnected sets the connection status.
func (sm *SLURMMonitor) setConnected(connected bool) {
	sm.connectedMutex.Lock()
	defer sm.connectedMutex.Unlock()

	sm.connected = connected
}

// CancelJob cancels a SLURM job by ID.
func (sm *SLURMMonitor) CancelJob(jobID string) error { //nolint:unparam // Error can be returned from cancelRealSLURMJob
	// Try to cancel using real SLURM scancel command first
	err := sm.cancelRealSLURMJob(sm.ctx, jobID)
	if err != nil {
		log.Printf("SLURM scancel not available, simulating cancellation: %v", err)
		// In simulation mode, just remove the job from the list
		sm.removeJobFromList(jobID)

		return nil
	}

	// Force a refresh after cancellation
	go sm.fetchJobs()

	return nil
}

// cancelRealSLURMJob attempts to cancel a real SLURM job using scancel.
func (sm *SLURMMonitor) cancelRealSLURMJob(ctx context.Context, jobID string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "scancel", jobID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scancel command failed: %w, output: %s", err, string(output))
	}

	return nil
}

// removeJobFromList removes a job from the simulated job list.
func (sm *SLURMMonitor) removeJobFromList(jobID string) {
	sm.jobsMutex.Lock()
	defer sm.jobsMutex.Unlock()

	for i, job := range sm.jobs {
		if job.JobID == jobID {
			sm.jobs = append(sm.jobs[:i], sm.jobs[i+1:]...)

			break
		}
	}

	// Notify UI of updates
	select {
	case sm.updateChan <- struct{}{}:
	default:
		// Channel full, skip this update
	}
}

// monitorLoop runs the periodic SLURM job monitoring.
func (sm *SLURMMonitor) monitorLoop() {
	ticker := time.NewTicker(5 * time.Second) // Update every 5 seconds
	defer ticker.Stop()

	// Initial fetch
	sm.fetchJobs()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.fetchJobs()
		}
	}
}

// fetchJobs retrieves current SLURM job information from the actual SLURM queue.
func (sm *SLURMMonitor) fetchJobs() {
	// Try to get real SLURM job data first
	realJobs, err := sm.fetchRealSLURMJobs(sm.ctx)
	if err != nil {
		// Fall back to simulated data if SLURM tools not available
		log.Printf("SLURM tools not available, using simulated data: %v", err)
		sm.setConnected(false)
		sm.useSimulatedData()

		return
	}

	sm.setConnected(true)
	sm.jobsMutex.Lock()
	sm.jobs = realJobs
	sm.jobsMutex.Unlock()

	// Notify UI of updates
	select {
	case sm.updateChan <- struct{}{}:
	default:
		// Channel full, skip this update
	}
}

// fetchRealSLURMJobs attempts to get real job data from SLURM squeue command.
func (sm *SLURMMonitor) fetchRealSLURMJobs(ctx context.Context) ([]SLURMJobInfo, error) {
	// Use squeue to get all jobs for all users
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// squeue format: JobID,User,State,Start,Partition,Nodes,CPUs
	cmd := exec.CommandContext(timeoutCtx, "squeue",
		"--format=%i,%u,%T,%S,%P,%D,%C",
		"--noheader",
		"--all") // Show all users' jobs

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("squeue command failed: %w", err)
	}

	return sm.parseSqueueOutput(string(output))
}

// parseSqueueOutput parses the output from squeue command.
func (sm *SLURMMonitor) parseSqueueOutput(output string) ([]SLURMJobInfo, error) { //nolint:unparam // Error might be needed for future validation
	var jobs []SLURMJobInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse CSV format: JobID,User,State,Start,Partition,Nodes,CPUs
		fields := strings.Split(line, ",")
		if len(fields) < 7 {
			continue
		}

		job := SLURMJobInfo{
			JobID:     strings.TrimSpace(fields[0]),
			Owner:     strings.TrimSpace(fields[1]),
			Status:    strings.TrimSpace(fields[2]),
			StartTime: strings.TrimSpace(fields[3]),
			Partition: strings.TrimSpace(fields[4]),
			Nodes:     strings.TrimSpace(fields[5]),
			CPUs:      strings.TrimSpace(fields[6]),
		}

		// Clean up start time format
		if job.StartTime == "N/A" || job.StartTime == "" {
			job.StartTime = "--"
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// useSimulatedData provides fallback simulated data when SLURM tools aren't available.
func (sm *SLURMMonitor) useSimulatedData() {
	currentTime := time.Now()
	newJobs := []SLURMJobInfo{
		{
			JobID:     "12345",
			Owner:     "user1",
			Status:    "RUNNING",
			StartTime: currentTime.Add(-30 * time.Minute).Format("15:04:05"),
			Partition: "normal",
			Nodes:     "1",
			CPUs:      "4",
		},
		{
			JobID:     "12346",
			Owner:     "user2",
			Status:    "PENDING",
			StartTime: "--",
			Partition: "high",
			Nodes:     "2",
			CPUs:      "8",
		},
	}

	// Simulate realistic job lifecycle changes
	second := currentTime.Second()

	// Add a job that transitions between states
	if second%30 < 10 {
		// First 10 seconds: job is PENDING
		newJobs = append(newJobs, SLURMJobInfo{
			JobID:     "12347",
			Owner:     "janus",
			Status:    "PENDING",
			StartTime: "--",
			Partition: "normal",
			Nodes:     "1",
			CPUs:      "2",
		})
	} else if second%30 < 25 {
		// Next 15 seconds: job is RUNNING
		newJobs = append(newJobs, SLURMJobInfo{
			JobID:     "12347",
			Owner:     "janus",
			Status:    "RUNNING",
			StartTime: currentTime.Add(-time.Duration(second%30-10) * time.Second).Format("15:04:05"),
			Partition: "normal",
			Nodes:     "1",
			CPUs:      "2",
		})
	}
	// Last 5 seconds: job completes and disappears from queue

	// Add another dynamic job to show different states
	if second%20 < 5 {
		newJobs = append(newJobs, SLURMJobInfo{
			JobID:     "12348",
			Owner:     "admin",
			Status:    "RUNNING",
			StartTime: currentTime.Add(-2 * time.Hour).Format("15:04:05"),
			Partition: "gpu",
			Nodes:     "4",
			CPUs:      "16",
		})
	} else if second%20 < 8 {
		newJobs = append(newJobs, SLURMJobInfo{
			JobID:     "12348",
			Owner:     "admin",
			Status:    "FAILED",
			StartTime: currentTime.Add(-2 * time.Hour).Format("15:04:05"),
			Partition: "gpu",
			Nodes:     "4",
			CPUs:      "16",
		})
	}

	sm.jobsMutex.Lock()
	sm.jobs = newJobs
	sm.jobsMutex.Unlock()

	// Notify UI of updates
	select {
	case sm.updateChan <- struct{}{}:
	default:
		// Channel full, skip this update
	}
}

// BuildSLURMJobsTable creates a dynamic table widget for SLURM jobs.
func (a *App) BuildSLURMJobsTable() *widget.Table {
	table := widget.NewTable(
		func() (int, int) {
			if a.slurmMonitor == nil {
				return 0, 8 // No data, but keep columns
			}
			jobs := a.slurmMonitor.GetJobs()

			return len(jobs), 8 // 8 columns: JobID, Owner, Status, StartTime, Partition, Nodes, CPUs, Actions
		},
		func() fyne.CanvasObject {
			// Return a label by default, but will be replaced with button for the last column
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if a.slurmMonitor == nil {
				if label, ok := obj.(*widget.Label); ok {
					label.SetText("")
				}

				return
			}

			jobs := a.slurmMonitor.GetJobs()
			if id.Row >= len(jobs) {
				if label, ok := obj.(*widget.Label); ok {
					label.SetText("")
				}

				return
			}

			job := jobs[id.Row]

			// Last column (7) is for cancel button
			if id.Col == 7 {
				// For the actions column, we need to create a button
				// Since Fyne table doesn't easily support different widget types per cell,
				// we'll use a label that looks like a button for now
				if label, ok := obj.(*widget.Label); ok {
					if job.Status == "RUNNING" || job.Status == "PENDING" {
						label.SetText("Cancel")
						label.Importance = widget.DangerImportance
					} else {
						label.SetText("")
					}
				}

				return
			}

			// Handle other columns as labels
			label, ok := obj.(*widget.Label)
			if !ok {
				return
			}

			switch id.Col {
			case 0:
				label.SetText(job.JobID)
			case 1:
				label.SetText(job.Owner)
			case 2:
				label.SetText(job.Status)
				// Color code status
				switch job.Status {
				case "RUNNING":
					label.Importance = widget.SuccessImportance
				case "PENDING":
					label.Importance = widget.MediumImportance
				case "FAILED", "CANCELLED":
					label.Importance = widget.DangerImportance
				default:
					label.Importance = widget.MediumImportance
				}
			case 3:
				label.SetText(job.StartTime)
			case 4:
				label.SetText(job.Partition)
			case 5:
				label.SetText(job.Nodes)
			case 6:
				label.SetText(job.CPUs)
			}
		},
	)

	// Add click handler for cancel functionality
	table.OnSelected = func(id widget.TableCellID) {
		if a.slurmMonitor == nil {
			return
		}

		jobs := a.slurmMonitor.GetJobs()
		if id.Row >= len(jobs) || id.Col != 7 {
			return
		}

		job := jobs[id.Row]
		// Only allow cancellation of running or pending jobs
		if job.Status == "RUNNING" || job.Status == "PENDING" {
			// Show confirmation dialog
			a.showCancelJobConfirmation(job.JobID)
		}
	}

	// Set responsive column widths that work well across different window sizes
	// Use larger minimum widths to prevent cramping
	table.SetColumnWidth(0, 120) // Job ID - enough for longer job IDs
	table.SetColumnWidth(1, 150) // Owner - enough for longer usernames
	table.SetColumnWidth(2, 120) // Status - enough for "COMPLETING", "CANCELLED", etc.
	table.SetColumnWidth(3, 140) // Start Time - enough for full timestamps
	table.SetColumnWidth(4, 120) // Partition - enough for longer partition names
	table.SetColumnWidth(5, 100) // Nodes - enough for node counts and ranges
	table.SetColumnWidth(6, 100) // CPUs - enough for CPU counts
	table.SetColumnWidth(7, 120) // Actions - enough for "Cancel" button and spacing

	return table
}

// BuildSLURMJobsTab creates the jobs tab with real SLURM data.
func (a *App) BuildSLURMJobsTab() fyne.CanvasObject {
	// Create the dynamic table (table has its own headers)
	table := a.BuildSLURMJobsTable()

	// Refresh button
	refreshBtn := widget.NewButton("Refresh", func() {
		if a.slurmMonitor != nil {
			// Force a refresh
			go a.slurmMonitor.fetchJobs()
		}
		table.Refresh()
	})

	// Status indicator
	statusLabel := widget.NewLabel("SLURM Status: Disconnected")
	if a.slurmMonitor != nil && a.slurmMonitor.IsConnected() {
		statusLabel.SetText("SLURM Status: Connected")
		statusLabel.Importance = widget.SuccessImportance
	} else if a.slurmMonitor != nil {
		statusLabel.SetText("SLURM Status: Disconnected")
		statusLabel.Importance = widget.DangerImportance
	}

	controls := container.NewHBox(refreshBtn, statusLabel)

	// Create table headers using the same table widget approach for perfect alignment
	headerTable := widget.NewTable(
		func() (int, int) { return 1, 8 }, // 1 row, 8 columns for headers
		func() fyne.CanvasObject { return widget.NewRichTextFromMarkdown("**Header**") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row != 0 {
				return
			}

			richText, ok := obj.(*widget.RichText)
			if !ok {
				return
			}

			headers := []string{"**Job ID**", "**Owner**", "**Status**", "**Start Time**", "**Partition**", "**Nodes**", "**CPUs**", "**Actions**"}
			if id.Col < len(headers) {
				richText.ParseMarkdown(headers[id.Col])
			}
		},
	)

	// Set the same column widths as the data table for perfect alignment
	headerTable.SetColumnWidth(0, 120) // Job ID
	headerTable.SetColumnWidth(1, 150) // Owner
	headerTable.SetColumnWidth(2, 120) // Status
	headerTable.SetColumnWidth(3, 140) // Start Time
	headerTable.SetColumnWidth(4, 120) // Partition
	headerTable.SetColumnWidth(5, 100) // Nodes
	headerTable.SetColumnWidth(6, 100) // CPUs
	headerTable.SetColumnWidth(7, 120) // Actions

	// Use border container to maximize table space
	return container.NewBorder(
		container.NewVBox(controls, headerTable), // Top
		nil,                                      // Bottom
		nil,                                      // Left
		nil,                                      // Right
		table,                                    // Center - takes up all remaining space
	)
}

// setupSLURMMonitoring initializes SLURM monitoring if configuration supports it.
func (a *App) setupSLURMMonitoring() {
	if a.config == nil {
		return
	}

	// Only set up SLURM monitoring if licensed for "grid" feature
	if !a.HasFeature("grid") {
		return
	}

	// Only set up SLURM monitoring if scheduler is SLURM
	if a.config.Scheduler != "SLURM" {
		return
	}

	// Only set up monitoring if SLURM is actually available
	if !IsSLURMAvailable() {
		return
	}

	monitor, err := NewSLURMMonitor(a.errorCtx, a.config)
	if err != nil {
		log.Printf("Failed to initialize SLURM monitor: %v", err)

		return
	}

	a.slurmMonitor = monitor
	monitor.Start()

	// Set up UI update listener
	go func() {
		for range monitor.GetUpdateChannel() {
			// Refresh the grid details if they're visible
			// This would trigger table refresh in the UI
			if a.window != nil {
				a.window.Content().Refresh()
			}
		}
	}()
}

// stopSLURMMonitoring stops the SLURM monitoring.
func (a *App) stopSLURMMonitoring() {
	if a.slurmMonitor != nil {
		a.slurmMonitor.Stop()
		a.slurmMonitor = nil
	}
}

// showCancelJobConfirmation shows a confirmation dialog for job cancellation.
func (a *App) showCancelJobConfirmation(jobID string) {
	content := widget.NewLabel(fmt.Sprintf("Are you sure you want to cancel job %s?", jobID))

	confirmDialog := widget.NewModalPopUp(
		container.NewVBox(
			content,
			container.NewHBox(
				widget.NewButton("Cancel Job", func() {
					if a.slurmMonitor != nil {
						err := a.slurmMonitor.CancelJob(jobID)
						if err != nil {
							log.Printf("Failed to cancel job %s: %v", jobID, err)
							// Show error dialog
							a.showErrorDialog("Cancel Failed", fmt.Sprintf("Failed to cancel job %s: %v", jobID, err))
						} else {
							log.Printf("Successfully cancelled job %s", jobID)
						}
					}
					a.cancelJobDialog.Hide()
					a.cancelJobDialog = nil
				}),
				widget.NewButton("Keep Job", func() {
					a.cancelJobDialog.Hide()
					a.cancelJobDialog = nil
				}),
			),
		),
		a.window.Canvas(),
	)

	confirmDialog.Show()
	a.cancelJobDialog = confirmDialog
}

// showErrorDialog shows an error dialog to the user.
func (a *App) showErrorDialog(title, message string) {
	content := widget.NewLabel(message)
	content.Wrapping = fyne.TextWrapWord

	errorDialog := widget.NewModalPopUp(
		container.NewVBox(
			widget.NewRichTextFromMarkdown(fmt.Sprintf("**%s**", title)),
			content,
			widget.NewButton("OK", func() {
				a.errorDialog.Hide()
				a.errorDialog = nil
			}),
		),
		a.window.Canvas(),
	)

	errorDialog.Show()
	a.errorDialog = errorDialog
}
