package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (a *App) buildGridDetails() fyne.CanvasObject {
	// Scheduler dropdown
	schedulerSelect := widget.NewSelect([]string{"SLURM", "SGE", "TORQUE"}, nil)
	defaultScheduler := "SLURM"
	if a.config != nil && a.config.Scheduler != "" {
		defaultScheduler = a.config.Scheduler
	}
	schedulerSelect.SetSelected(defaultScheduler)

	header := container.NewBorder(
		nil, nil,
		widget.NewLabel("Grid Details"),
		schedulerSelect,
	)

	// Jobs and Nodes tabs
	jobsTab := a.buildJobsTab()
	nodesTab := a.buildNodesTab()

	gridTabs := container.NewAppTabs(
		container.NewTabItem("Jobs", jobsTab),
		container.NewTabItem("Nodes", nodesTab),
	)

	// Create grid section without height restrictions to allow full expansion
	gridSection := container.NewVBox(
		header,
		widget.NewSeparator(),
		gridTabs,
	)

	return gridSection
}

func (a *App) buildJobsTab() fyne.CanvasObject {
	// Check if SLURM monitoring is available
	if a.slurmMonitor != nil {
		return a.BuildSLURMJobsTab()
	}

	// Fallback to sample data if SLURM monitoring is not available
	jobData := [][]string{
		{"3", "johnd", "r", "2025-01-01 00:00:00"},
		{"4", "johnd", "cf", "2025-01-02 01:00:00"},
	}

	// Create table with sample data
	table := widget.NewTable(
		func() (int, int) { return len(jobData), 4 }, // rows, cols
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label, ok := obj.(*widget.Label)
			if !ok {
				return
			}
			if id.Row < len(jobData) && id.Col < len(jobData[id.Row]) {
				label.SetText(jobData[id.Row][id.Col])
			}
		},
	)

	// Set column headers
	table.SetColumnWidth(0, 60)  // Job ID
	table.SetColumnWidth(1, 80)  // Owner
	table.SetColumnWidth(2, 60)  // Status
	table.SetColumnWidth(3, 150) // Created

	// Header labels
	headers := container.NewHBox(
		widget.NewRichTextFromMarkdown("**Job ID**"),
		widget.NewRichTextFromMarkdown("**Owner**"),
		widget.NewRichTextFromMarkdown("**Status**"),
		widget.NewRichTextFromMarkdown("**Created**"),
	)

	// Add status message about SLURM monitoring
	statusMsg := widget.NewLabel("SLURM monitoring not configured. Showing sample data.")
	statusMsg.Importance = widget.WarningImportance

	return container.NewVBox(statusMsg, headers, table)
}

func (a *App) buildNodesTab() fyne.CanvasObject {
	return widget.NewLabel("Node status monitoring placeholder\n\n- Cluster capacity\n- Autoscaling status\n- Node health")
}
