package config

import "testing"

func TestPsNImageOrDefault(t *testing.T) {
	tests := []struct {
		name      string
		perModel  string
		global    string
		want      string
		wantError bool
	}{
		{name: "per-model wins over global", perModel: "ghcr.io/x/psn:1", global: "ghcr.io/x/psn:global", want: "ghcr.io/x/psn:1"},
		{name: "falls back to global", perModel: "", global: "ghcr.io/x/psn:global", want: "ghcr.io/x/psn:global"},
		{name: "per-model whitespace falls back to global", perModel: "   ", global: "ghcr.io/x/psn:global", want: "ghcr.io/x/psn:global"},
		{name: "error when neither set", perModel: "", global: "", wantError: true},
		{name: "error when both blank", perModel: "  ", global: "  ", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &HermesExecutionConfig{PsNImage: tt.perModel}
			got, err := c.PsNImageOrDefault(tt.global)

			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got image %q", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBootstrapParallelismOrDefault(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "unset uses default", in: 0, want: DefaultBootstrapParallelism},
		{name: "negative uses default", in: -5, want: DefaultBootstrapParallelism},
		{name: "positive is honored", in: 32, want: 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := HermesKubernetesConfig{BootstrapParallelism: tt.in}
			if got := k.BootstrapParallelismOrDefault(); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFitTimeoutDuration(t *testing.T) {
	t.Run("empty means no deadline", func(t *testing.T) {
		k := HermesKubernetesConfig{FitTimeout: ""}
		d, err := k.FitTimeoutDuration()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if d != 0 {
			t.Errorf("got %v, want 0", d)
		}
	})

	t.Run("valid duration parses", func(t *testing.T) {
		k := HermesKubernetesConfig{FitTimeout: "30m"}
		d, err := k.FitTimeoutDuration()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if d.Minutes() != 30 {
			t.Errorf("got %v, want 30m", d)
		}
	})

	t.Run("invalid duration errors", func(t *testing.T) {
		k := HermesKubernetesConfig{FitTimeout: "soon"}
		if _, err := k.FitTimeoutDuration(); err == nil {
			t.Fatal("expected error for invalid duration")
		}
	})
}

func TestValidateHermesKubernetesBootstrapFields(t *testing.T) {
	tests := []struct {
		name         string
		orchestrator string
		k8s          HermesKubernetesConfig
		wantErr      bool
	}{
		{
			name:         "valid bootstrap settings pass",
			orchestrator: OrchestratorKubernetes,
			k8s:          HermesKubernetesConfig{Namespace: "ns", BootstrapParallelism: 24, FitTimeout: "45m"},
		},
		{
			name:         "negative parallelism rejected",
			orchestrator: OrchestratorKubernetes,
			k8s:          HermesKubernetesConfig{Namespace: "ns", BootstrapParallelism: -1},
			wantErr:      true,
		},
		{
			name:         "invalid fit_timeout rejected",
			orchestrator: OrchestratorKubernetes,
			k8s:          HermesKubernetesConfig{Namespace: "ns", FitTimeout: "nope"},
			wantErr:      true,
		},
		{
			name:         "bad values ignored when not kubernetes orchestrator",
			orchestrator: OrchestratorDocker,
			k8s:          HermesKubernetesConfig{BootstrapParallelism: -1, FitTimeout: "nope"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHermesKubernetes(tt.orchestrator, tt.k8s)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
