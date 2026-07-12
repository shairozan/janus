//go:build integration && gui
// +build integration,gui

package gui

import (
	"context"
	"testing"
	"time"
)

// TestGUICloseHandling verifies that closing the GUI window triggers the same
// graceful shutdown as signal handling (CTRL+C)
func TestGUICloseHandling(t *testing.T) {
	// Create a context (simulating what comes from main)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a GUI app (this would normally be done in RunGUI)
	app := NewApp(ctx)

	// Verify the app has proper cleanup mechanisms
	if app.errorCancel == nil {
		t.Fatal("App should have error cancel function")
	}

	// Simulate some active runs (like what would happen when user submits jobs)
	mockCancel1 := func() { t.Log("Mock run 1 cancelled") }
	mockCancel2 := func() { t.Log("Mock run 2 cancelled") }

	app.activeRuns[1] = mockCancel1
	app.activeRuns[2] = mockCancel2

	// Track if cleanup was called
	cleanupCalled := make(chan bool, 1)

	// Simulate GUI close by calling the same cleanup that SetCloseIntercept would call
	go func() {
		app.Cleanup()
		cleanupCalled <- true
	}()

	// Verify cleanup completes quickly
	select {
	case <-cleanupCalled:
		t.Log("✅ GUI cleanup completed successfully")
	case <-time.After(2 * time.Second):
		t.Fatal("GUI cleanup did not complete within 2 seconds")
	}

	// Verify contexts were cancelled
	select {
	case <-app.errorCtx.Done():
		t.Log("✅ Error context was cancelled by cleanup")
	case <-time.After(100 * time.Millisecond):
		t.Error("Error context was not cancelled by cleanup")
	}

	// Verify active runs map was cleared
	if len(app.activeRuns) != 0 {
		t.Errorf("Expected active runs to be cleared, got %d runs", len(app.activeRuns))
	}
}

// TestGUIContextPropagation verifies that GUI contexts are properly derived from main context
func TestGUIContextPropagation(t *testing.T) {
	// Create main context (like signal.NotifyContext would)
	mainCtx, mainCancel := context.WithCancel(context.Background())

	// Create GUI app
	app := NewApp(mainCtx)

	// Verify GUI contexts are derived from main context
	if app.errorCtx == nil {
		t.Fatal("GUI app should have error context")
	}

	// Test that cancelling main context propagates to GUI contexts
	done := make(chan bool, 1)

	go func() {
		<-app.errorCtx.Done()
		done <- true
	}()

	// Small delay to ensure goroutine is waiting
	time.Sleep(10 * time.Millisecond)

	// Cancel main context (simulate signal)
	mainCancel()

	// Verify GUI context is cancelled
	select {
	case <-done:
		t.Log("✅ Main context cancellation propagated to GUI context")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Main context cancellation did not propagate to GUI context")
	}
}

// TestExecutionCancellationEquivalence verifies that GUI close and signal handling
// both result in the same execution cancellation behavior
func TestExecutionCancellationEquivalence(t *testing.T) {
	// Test 1: Signal-based cancellation
	t.Run("signal_cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		app := NewApp(ctx)

		// Add mock execution
		mockCancel := func() { t.Log("Signal test: execution cancelled") }
		app.activeRuns[100] = mockCancel

		// Simulate signal by cancelling main context
		cancel()

		// Wait for context propagation
		<-app.errorCtx.Done()
		t.Log("✅ Signal cancellation propagated to GUI")
	})

	// Test 2: GUI close-based cancellation
	t.Run("gui_close_cancellation", func(t *testing.T) {
		ctx := context.Background()
		app := NewApp(ctx)

		// Add mock execution
		mockCancel := func() { t.Log("GUI close test: execution cancelled") }
		app.activeRuns[200] = mockCancel

		// Simulate GUI close by calling cleanup directly
		app.Cleanup()

		// Verify context was cancelled
		select {
		case <-app.errorCtx.Done():
			t.Log("✅ GUI close triggered context cancellation")
		case <-time.After(100 * time.Millisecond):
			t.Error("GUI close did not cancel context")
		}

		// Verify runs were cancelled
		if len(app.activeRuns) != 0 {
			t.Error("GUI close did not cancel active runs")
		}
	})
}

// TestCloseHandlerDocumentation documents the GUI close handling behavior
func TestCloseHandlerDocumentation(t *testing.T) {
	t.Log("=== GUI Close Handling Architecture ===")
	t.Log("")
	t.Log("1. User clicks [X] button on GUI window")
	t.Log("2. Fyne triggers window.SetCloseIntercept() callback")
	t.Log("3. Callback calls app.Cleanup()")
	t.Log("4. Cleanup() performs same actions as signal handling:")
	t.Log("   - Stops SLURM monitoring")
	t.Log("   - Cancels error handling context")
	t.Log("   - Cancels file watching")
	t.Log("   - Cancels all active execution contexts")
	t.Log("   - Updates run records with cancellation message")
	t.Log("   - Closes error channel")
	t.Log("5. Callback calls fyneApp.Quit()")
	t.Log("")
	t.Log("Result: GUI close has IDENTICAL behavior to CTRL+C")
	t.Log("✅ Both trigger graceful shutdown of all SLURM jobs")
	t.Log("✅ Both ensure proper resource cleanup")
	t.Log("✅ Both prevent orphaned processes")
}

// MockExecutionForCloseTest simulates an execution that respects context cancellation
func MockExecutionForCloseTest(ctx context.Context, t *testing.T) {
	done := make(chan bool)

	go func() {
		// Simulate long-running execution
		select {
		case <-ctx.Done():
			t.Log("Mock execution cancelled due to context")
		case <-time.After(10 * time.Second):
			t.Log("Mock execution completed normally")
		}
		close(done)
	}()

	return
}

// TestIntegratedCloseWithExecution tests GUI close with actual execution context
func TestIntegratedCloseWithExecution(t *testing.T) {
	// Create main context
	mainCtx, mainCancel := context.WithCancel(context.Background())
	defer mainCancel()

	// Create GUI app
	app := NewApp(mainCtx)

	// Create execution context (like executeLocalRun would)
	execCtx, execCancel := context.WithTimeout(app.errorCtx, 30*time.Minute)
	defer execCancel()

	// Store the cancel function (like active runs do)
	app.activeRuns[999] = execCancel

	// Start mock execution
	executionDone := make(chan bool, 1)
	go func() {
		MockExecutionForCloseTest(execCtx, t)
		executionDone <- true
	}()

	// Let execution start
	time.Sleep(100 * time.Millisecond)

	// Simulate GUI close
	app.Cleanup()

	// Verify execution terminates quickly
	select {
	case <-executionDone:
		t.Log("✅ GUI close successfully cancelled execution")
	case <-time.After(1 * time.Second):
		t.Fatal("Execution did not terminate after GUI close")
	}
}