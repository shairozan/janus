package config

import "testing"

func TestValidateHermesKubernetes(t *testing.T) {
	tests := []struct {
		name         string
		orchestrator string
		k8s          HermesKubernetesConfig
		wantErr      bool
	}{
		{
			name:         "docker orchestrator ignores kubernetes settings",
			orchestrator: OrchestratorDocker,
			k8s:          HermesKubernetesConfig{},
			wantErr:      false,
		},
		{
			name:         "kubernetes requires a namespace",
			orchestrator: OrchestratorKubernetes,
			k8s:          HermesKubernetesConfig{},
			wantErr:      true,
		},
		{
			name:         "kubernetes with a namespace is valid",
			orchestrator: OrchestratorKubernetes,
			k8s:          HermesKubernetesConfig{Namespace: "janus"},
			wantErr:      false,
		},
		{
			name:         "blank namespace is rejected",
			orchestrator: OrchestratorKubernetes,
			k8s:          HermesKubernetesConfig{Namespace: "   "},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHermesKubernetes(tt.orchestrator, tt.k8s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHermesKubernetes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeriveExecutionMode(t *testing.T) {
	tests := []struct {
		engine, destination, want string
	}{
		{EngineNONMEM, DestinationHere, ExecutionModeNONMEM},
		{EnginePSN, DestinationScheduler, ExecutionModePSN},
		{EngineBBI, DestinationHere, ExecutionModeBBI},
		// Any engine on Hermes runs via the HERMES executor.
		{EngineNONMEM, DestinationHermes, ExecutionModeHERMES},
		{EnginePSN, DestinationHermes, ExecutionModeHERMES},
		{EngineBBI, DestinationHermes, ExecutionModeHERMES},
	}

	for _, tt := range tests {
		if got := DeriveExecutionMode(tt.engine, tt.destination); got != tt.want {
			t.Errorf("DeriveExecutionMode(%q, %q) = %q, want %q", tt.engine, tt.destination, got, tt.want)
		}
	}
}

func TestNormalizeExecutionAxes_FromLegacy(t *testing.T) {
	tests := []struct {
		name                                     string
		in                                       Input
		wantEngine, wantDest, wantOrch, wantMode string
		wantScheduler                            string
	}{
		{
			name:       "legacy NONMEM local",
			in:         Input{ExecutionMode: ExecutionModeNONMEM, Scheduler: "LOCAL"},
			wantEngine: EngineNONMEM, wantDest: DestinationHere, wantOrch: "",
			wantMode: ExecutionModeNONMEM, wantScheduler: "LOCAL",
		},
		{
			name:       "legacy PSN on a scheduler",
			in:         Input{ExecutionMode: ExecutionModePSN, Scheduler: "SLURM"},
			wantEngine: EnginePSN, wantDest: DestinationScheduler, wantOrch: "",
			wantMode: ExecutionModePSN, wantScheduler: "SLURM",
		},
		{
			name:       "legacy HERMES",
			in:         Input{ExecutionMode: ExecutionModeHERMES, Scheduler: "LOCAL"},
			wantEngine: EngineNONMEM, wantDest: DestinationHermes, wantOrch: OrchestratorDocker,
			wantMode: ExecutionModeHERMES, wantScheduler: "LOCAL",
		},
		{
			name:       "legacy empty scheduler is local",
			in:         Input{ExecutionMode: ExecutionModeBBI},
			wantEngine: EngineBBI, wantDest: DestinationHere, wantOrch: "",
			wantMode: ExecutionModeBBI, wantScheduler: "",
		},
		{
			name:       "invalid mode is left for validation to reject",
			in:         Input{ExecutionMode: "BOGUS"},
			wantEngine: EngineNONMEM, wantDest: DestinationHere, wantOrch: "",
			wantMode: "BOGUS", wantScheduler: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in
			normalizeExecutionAxes(&in)

			if in.Engine != tt.wantEngine {
				t.Errorf("Engine = %q, want %q", in.Engine, tt.wantEngine)
			}
			if in.Destination != tt.wantDest {
				t.Errorf("Destination = %q, want %q", in.Destination, tt.wantDest)
			}
			if in.Orchestrator != tt.wantOrch {
				t.Errorf("Orchestrator = %q, want %q", in.Orchestrator, tt.wantOrch)
			}
			if in.ExecutionMode != tt.wantMode {
				t.Errorf("ExecutionMode = %q, want %q", in.ExecutionMode, tt.wantMode)
			}
			if in.Scheduler != tt.wantScheduler {
				t.Errorf("Scheduler = %q, want %q", in.Scheduler, tt.wantScheduler)
			}
		})
	}
}

func TestNormalizeExecutionAxes_AxesAuthoritative(t *testing.T) {
	tests := []struct {
		name             string
		in               Input
		wantMode         string
		wantScheduler    string
		wantOrchestrator string
	}{
		{
			name:          "engine + Here derives mode and forces local scheduler",
			in:            Input{Engine: EnginePSN, Destination: DestinationHere, Scheduler: "SLURM"},
			wantMode:      ExecutionModePSN,
			wantScheduler: "LOCAL",
		},
		{
			name:          "engine + Scheduler keeps the scheduler",
			in:            Input{Engine: EngineNONMEM, Destination: DestinationScheduler, Scheduler: "SGE"},
			wantMode:      ExecutionModeNONMEM,
			wantScheduler: "SGE",
		},
		{
			name:             "Hermes derives HERMES mode and defaults orchestrator to Docker",
			in:               Input{Engine: EngineBBI, Destination: DestinationHermes},
			wantMode:         ExecutionModeHERMES,
			wantScheduler:    "LOCAL",
			wantOrchestrator: OrchestratorDocker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in
			normalizeExecutionAxes(&in)

			if in.ExecutionMode != tt.wantMode {
				t.Errorf("ExecutionMode = %q, want %q", in.ExecutionMode, tt.wantMode)
			}
			if in.Scheduler != tt.wantScheduler {
				t.Errorf("Scheduler = %q, want %q", in.Scheduler, tt.wantScheduler)
			}
			if tt.wantOrchestrator != "" && in.Orchestrator != tt.wantOrchestrator {
				t.Errorf("Orchestrator = %q, want %q", in.Orchestrator, tt.wantOrchestrator)
			}
		})
	}
}
