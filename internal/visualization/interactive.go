package visualization

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	echartOpts "github.com/go-echarts/go-echarts/v2/opts"
)

// InteractiveChartOptions configures interactive HTML chart generation.
type InteractiveChartOptions struct {
	Title  string
	Width  string // CSS width (e.g., "100%", "800px")
	Height string // CSS height (e.g., "500px")
	Theme  string // ECharts theme (e.g., "dark", "vintage")
}

// DefaultInteractiveOptions returns sensible defaults for interactive charts.
func DefaultInteractiveOptions() InteractiveChartOptions {
	return InteractiveChartOptions{
		Width:  "100%",
		Height: "500px",
		Theme:  "",
	}
}

// GenerateInteractiveOFVChart creates an interactive HTML line chart for OFV trend.
func GenerateInteractiveOFVChart(data []OFVDataPoint, opts InteractiveChartOptions) (*charts.Line, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data points provided")
	}

	// Sort by timestamp
	sortedData := make([]OFVDataPoint, len(data))
	copy(sortedData, data)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})

	// Extract labels and values
	xAxis := make([]string, len(sortedData))
	yData := make([]echartOpts.LineData, len(sortedData))
	for i, dp := range sortedData {
		xAxis[i] = dp.Label
		yData[i] = echartOpts.LineData{Value: dp.OFV}
	}

	// Create line chart
	line := charts.NewLine()

	// Set global options
	title := opts.Title
	if title == "" {
		title = "OFV Trend Across Runs"
	}

	line.SetGlobalOptions(
		charts.WithTitleOpts(echartOpts.Title{
			Title:    title,
			Subtitle: "Objective Function Value progression",
		}),
		charts.WithTooltipOpts(echartOpts.Tooltip{
			Show:    echartOpts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithLegendOpts(echartOpts.Legend{
			Show: echartOpts.Bool(true),
		}),
		charts.WithDataZoomOpts(echartOpts.DataZoom{
			Type:  "slider",
			Start: 0,
			End:   100,
		}),
		charts.WithToolboxOpts(echartOpts.Toolbox{
			Show: echartOpts.Bool(true),
			Feature: &echartOpts.ToolBoxFeature{
				SaveAsImage: &echartOpts.ToolBoxFeatureSaveAsImage{
					Show:  echartOpts.Bool(true),
					Title: "Save as PNG",
				},
				DataZoom: &echartOpts.ToolBoxFeatureDataZoom{
					Show: echartOpts.Bool(true),
				},
				Restore: &echartOpts.ToolBoxFeatureRestore{
					Show: echartOpts.Bool(true),
				},
			},
		}),
	)

	// Add data
	line.SetXAxis(xAxis).
		AddSeries("OFV", yData).
		SetSeriesOptions(
			charts.WithLineChartOpts(echartOpts.LineChart{
				Smooth:     echartOpts.Bool(true),
				ShowSymbol: echartOpts.Bool(true),
			}),
			charts.WithMarkPointNameTypeItemOpts(
				echartOpts.MarkPointNameTypeItem{Name: "Maximum", Type: "max"},
				echartOpts.MarkPointNameTypeItem{Name: "Minimum", Type: "min"},
			),
			charts.WithMarkLineNameTypeItemOpts(
				echartOpts.MarkLineNameTypeItem{Name: "Average", Type: "average"},
			),
		)

	return line, nil
}

// GenerateInteractiveParameterChart creates an interactive HTML bar chart for parameters.
func GenerateInteractiveParameterChart(data []ParameterDataPoint, options InteractiveChartOptions) (*charts.Bar, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data points provided")
	}

	// Sort by timestamp
	sortedData := make([]ParameterDataPoint, len(data))
	copy(sortedData, data)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})

	// Extract labels and values
	xAxis := make([]string, len(sortedData))
	yData := make([]echartOpts.BarData, len(sortedData))
	for i, dp := range sortedData {
		xAxis[i] = dp.Label
		if dp.Value != nil {
			yData[i] = echartOpts.BarData{Value: *dp.Value}
		} else {
			yData[i] = echartOpts.BarData{Value: 0}
		}
	}

	// Create bar chart
	bar := charts.NewBar()

	title := options.Title
	if title == "" && len(data) > 0 {
		title = fmt.Sprintf("%s Across Runs", data[0].Name)
	}

	bar.SetGlobalOptions(
		charts.WithTitleOpts(echartOpts.Title{
			Title: title,
		}),
		charts.WithTooltipOpts(echartOpts.Tooltip{
			Show:    echartOpts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithToolboxOpts(echartOpts.Toolbox{
			Show: echartOpts.Bool(true),
			Feature: &echartOpts.ToolBoxFeature{
				SaveAsImage: &echartOpts.ToolBoxFeatureSaveAsImage{
					Show:  echartOpts.Bool(true),
					Title: "Save as PNG",
				},
			},
		}),
	)

	paramName := "Value"
	if len(data) > 0 {
		paramName = data[0].Name
	}

	bar.SetXAxis(xAxis).
		AddSeries(paramName, yData).
		SetSeriesOptions(
			charts.WithLabelOpts(echartOpts.Label{
				Show:     echartOpts.Bool(true),
				Position: "top",
			}),
		)

	return bar, nil
}

// GenerateInteractiveMultiParameterChart creates an interactive grouped bar chart.
func GenerateInteractiveMultiParameterChart(paramNames []string, dataByParam map[string][]ParameterDataPoint, options InteractiveChartOptions) (*charts.Bar, error) {
	if len(paramNames) == 0 {
		return nil, fmt.Errorf("no parameters specified")
	}

	// Get run labels from first parameter that has data
	var xAxis []string
	for _, name := range paramNames {
		if data, ok := dataByParam[name]; ok && len(data) > 0 {
			sortedData := make([]ParameterDataPoint, len(data))
			copy(sortedData, data)
			sort.Slice(sortedData, func(i, j int) bool {
				return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
			})

			for _, dp := range sortedData {
				xAxis = append(xAxis, dp.Label)
			}

			break
		}
	}

	if len(xAxis) == 0 {
		return nil, fmt.Errorf("no data available")
	}

	// Create bar chart
	bar := charts.NewBar()

	title := options.Title
	if title == "" {
		title = "Parameter Comparison"
	}

	bar.SetGlobalOptions(
		charts.WithTitleOpts(echartOpts.Title{
			Title: title,
		}),
		charts.WithTooltipOpts(echartOpts.Tooltip{
			Show:    echartOpts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithLegendOpts(echartOpts.Legend{
			Show: echartOpts.Bool(true),
		}),
		charts.WithToolboxOpts(echartOpts.Toolbox{
			Show: echartOpts.Bool(true),
			Feature: &echartOpts.ToolBoxFeature{
				SaveAsImage: &echartOpts.ToolBoxFeatureSaveAsImage{
					Show: echartOpts.Bool(true),
				},
			},
		}),
	)

	bar.SetXAxis(xAxis)

	// Add series for each parameter
	for _, name := range paramNames {
		data := dataByParam[name]
		sortedData := make([]ParameterDataPoint, len(data))
		copy(sortedData, data)
		sort.Slice(sortedData, func(i, j int) bool {
			return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
		})

		// Build value map
		dataMap := make(map[string]*float64)
		for _, dp := range sortedData {
			dataMap[dp.Label] = dp.Value
		}

		// Create bar data aligned to xAxis
		yData := make([]echartOpts.BarData, len(xAxis))
		for j, label := range xAxis {
			if val, ok := dataMap[label]; ok && val != nil {
				yData[j] = echartOpts.BarData{Value: *val}
			} else {
				yData[j] = echartOpts.BarData{Value: 0}
			}
		}

		bar.AddSeries(name, yData)
	}

	return bar, nil
}

// Renderable is an interface for go-echarts chart types that can render to a writer.
type Renderable interface {
	Render(w io.Writer) error
}

// ServeInteractiveChart renders an interactive chart on the local chart server
// and opens it in the default browser. It is the generic entry point behind the
// GenerateAndOpenInteractive* helpers, for callers that build their own chart.
func ServeInteractiveChart(chart Renderable) error {
	return serveChart(chart, "")
}

// SaveInteractiveChart saves an interactive chart to an HTML file.
func SaveInteractiveChart(chart Renderable, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := chart.Render(f); err != nil {
		return fmt.Errorf("failed to render chart: %w", err)
	}

	return nil
}

// chartServer manages a single HTTP server for interactive charts.
type chartServer struct {
	mu          sync.RWMutex
	server      *http.Server
	listener    net.Listener
	chartHTML   []byte
	running     bool
	lastAccess  time.Time
	shutdownCtx context.Context //nolint:containedctx // Lifecycle management for long-lived server
	cancelFunc  context.CancelFunc
}

// Global single chart server instance.
var globalChartServer = &chartServer{}

const chartServerTimeout = 10 * time.Minute

// serveChart updates the chart content and ensures the server is running.
func serveChart(chart Renderable, _ string) error {
	// Render chart to buffer
	var buf bytes.Buffer
	if err := chart.Render(&buf); err != nil {
		return fmt.Errorf("failed to render chart: %w", err)
	}

	cs := globalChartServer
	cs.mu.Lock()

	// Update chart content
	cs.chartHTML = buf.Bytes()
	cs.lastAccess = time.Now()

	// Start server if not running
	if !cs.running {
		if err := cs.startLocked(); err != nil {
			cs.mu.Unlock()

			return err
		}
	}

	addr, _ := cs.listener.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://127.0.0.1:%d", addr.Port)
	cs.mu.Unlock()

	// Open browser
	if err := openURL(url); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

// startLocked starts the server. Must be called with mu held.
func (cs *chartServer) startLocked() error {
	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}

	cs.listener = listener
	cs.shutdownCtx, cs.cancelFunc = context.WithCancel(context.Background())

	// Create HTTP handler that serves current chart content
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		cs.mu.RLock()
		html := cs.chartHTML
		cs.lastAccess = time.Now()
		cs.mu.RUnlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(html)
	})

	cs.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	cs.running = true

	// Start server in background
	go func() {
		_ = cs.server.Serve(listener)
	}()

	// Start idle timeout monitor
	go cs.monitorIdleTimeout()

	return nil
}

// monitorIdleTimeout shuts down the server after period of inactivity.
func (cs *chartServer) monitorIdleTimeout() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-cs.shutdownCtx.Done():
			return
		case <-ticker.C:
			cs.mu.RLock()
			idle := time.Since(cs.lastAccess)
			cs.mu.RUnlock()

			if idle >= chartServerTimeout {
				cs.Shutdown()

				return
			}
		}
	}
}

// Shutdown gracefully shuts down the chart server.
func (cs *chartServer) Shutdown() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.running {
		return
	}

	if cs.cancelFunc != nil {
		cs.cancelFunc()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if cs.server != nil {
		_ = cs.server.Shutdown(ctx)
	}

	cs.running = false
	cs.server = nil
	cs.listener = nil
}

// ShutdownChartServer shuts down the global chart server.
// Call this when the application is closing.
func ShutdownChartServer() {
	globalChartServer.Shutdown()
}

// ShutdownAllChartServers is an alias for ShutdownChartServer for backwards compatibility.
func ShutdownAllChartServers() {
	ShutdownChartServer()
}

// openURL opens the given URL in the default browser.
func openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // Linux and others
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

// OpenInBrowser opens the given file path in the default browser.
//
// Deprecated: Use serveChart for interactive charts instead.
func OpenInBrowser(filePath string) error {
	var absPath string
	if filePath[0] != '/' {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		absPath = cwd + "/" + filePath
	} else {
		absPath = filePath
	}

	return openURL("file://" + absPath)
}

// GenerateAndOpenInteractiveOFV generates an OFV trend chart and opens it in browser.
func GenerateAndOpenInteractiveOFV(data []OFVDataPoint, options InteractiveChartOptions) error {
	chart, err := GenerateInteractiveOFVChart(data, options)
	if err != nil {
		return err
	}

	title := options.Title
	if title == "" {
		title = "OFV Trend"
	}

	return serveChart(chart, title)
}

// GenerateAndOpenInteractiveParameter generates a parameter chart and opens it in browser.
func GenerateAndOpenInteractiveParameter(data []ParameterDataPoint, options InteractiveChartOptions) error {
	chart, err := GenerateInteractiveParameterChart(data, options)
	if err != nil {
		return err
	}

	title := options.Title
	if title == "" && len(data) > 0 {
		title = data[0].Name
	}

	return serveChart(chart, title)
}
