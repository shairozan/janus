# KASM-Based Janus Trial Service

## Overview

This feature provides a zero-installation trial experience for Janus using KASM Workspaces, allowing potential users to evaluate Janus in a fully configured environment accessed through their web browser. Users can request a 15-minute trial session, receive a pre-configured Debian desktop with Janus and Hermes already installed, and only need to drag-and-drop their NONMEM license to begin executing models.

**Key Value Proposition**: Eliminates all setup friction (Docker, NONMEM installation, Hermes image acquisition, license placement) and enables instant demonstration and evaluation of Janus capabilities.

## Motivation

### Current Barriers to Evaluation

**Setup Complexity for New Users**:
1. Obtain NONMEM license from Icon
2. Install NONMEM (complex, platform-dependent) OR install Docker
3. Configure Docker socket access
4. Acquire Hermes container image (requires authentication/access)
5. Install Janus binary
6. Configure Janus to find NONMEM/Docker
7. Place license file in correct location

**Time Investment**: 1-4 hours depending on user experience level

**Failure Points**: Many users give up during Docker configuration or NONMEM installation

### Trial Service Solution

**User Experience**:
1. Visit trial website
2. Click "Request Trial"
3. Receive browser link to KASM desktop
4. Drag NONMEM license file onto desktop
5. Open Janus (already installed)
6. Execute models immediately

**Time to First Execution**: < 2 minutes (plus license drag-and-drop)

**Failure Points**: Eliminated (all infrastructure pre-configured)

## Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                    User's Web Browser                        │
│  https://try.janus.pharmalytica.com                         │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ HTTPS (WebSocket for VNC)
                 ▼
┌─────────────────────────────────────────────────────────────┐
│              Trial Provisioning Service                      │
│  - Request handling API                                      │
│  - KASM Workspace API client                                 │
│  - Session lifecycle management                              │
│  - Usage analytics                                           │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ KASM API (create/destroy workspaces)
                 ▼
┌─────────────────────────────────────────────────────────────┐
│                   KASM Workspaces Server                     │
│  - Workspace orchestration                                   │
│  - VNC/noVNC serving                                         │
│  - Container lifecycle management                            │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ Container Runtime
                 ▼
┌─────────────────────────────────────────────────────────────┐
│         Janus Trial Container (Debian Trixie)                │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ KASM Desktop (Debian Trixie + XFCE)                   │  │
│  │ ├── /usr/bin/janus (pre-installed)                    │  │
│  │ ├── /home/kasm-user/Desktop/ (NONMEM license target)  │  │
│  │ ├── /var/run/docker.sock (mounted from host)          │  │
│  │ └── Sample NONMEM models                              │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  Note: No Docker daemon IN container, socket mapped from    │
│        host to enable Hermes execution                      │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   │ Docker socket bind mount
                   ▼
┌─────────────────────────────────────────────────────────────┐
│                    Host Docker Daemon                        │
│  - Hermes execution containers                               │
│  - Pre-pulled hermes images                                  │
│  - Ephemeral execution (auto-cleanup)                        │
└─────────────────────────────────────────────────────────────┘
```

### KASM Desktop Container

**Base Image**: `kasmweb/debian-trixie-desktop:1.15.0` (or latest)

**Customizations**:
- Pre-install Janus DEB package
- Pre-install sample NONMEM models
- Desktop shortcuts for Janus and documentation
- Bind mount Docker socket from host
- Configure Janus to use Hermes execution mode
- Set license path to `~/Desktop/nonmem.lic`

**Docker Run Configuration**:
```bash
docker run -d \
  --name janus-trial-${SESSION_ID} \
  --shm-size=512m \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v trial-${SESSION_ID}-desktop:/home/kasm-user/Desktop \
  -e VNC_PW=password \
  -e KASM_PORT=6901 \
  -e JANUS_TRIAL_SESSION=true \
  -e JANUS_TRIAL_EXPIRES=$(date -d '+15 minutes' +%s) \
  --label com.pharmalytica.trial.session=${SESSION_ID} \
  --label com.pharmalytica.trial.expires=${EXPIRY_TIMESTAMP} \
  pharmalytica/janus-trial:latest
```

### Trial Container Dockerfile

**Location**: `installer/kasm/Dockerfile.trial`

```dockerfile
FROM kasmweb/debian-trixie-desktop:1.15.0

# Set environment to non-interactive for apt
ENV DEBIAN_FRONTEND=noninteractive

USER root

# Install Janus from DEB package
COPY dist/linux/janus_0.2.0_amd64.deb /tmp/
RUN apt-get update && \
    apt-get install -y /tmp/janus_0.2.0_amd64.deb && \
    rm /tmp/janus_0.2.0_amd64.deb && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Docker CLI (for Hermes interaction via host socket)
RUN apt-get update && \
    apt-get install -y \
        ca-certificates \
        curl \
        gnupg && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc && \
    chmod a+r /etc/apt/keyrings/docker.asc && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
          https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
          tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce-cli && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Add kasm-user to docker group (for socket access)
RUN usermod -aG docker kasm-user

# Copy sample NONMEM models
COPY testdata/acop /home/kasm-user/models/acop
RUN chown -R kasm-user:kasm-user /home/kasm-user/models

# Create Janus config file pre-configured for Hermes
RUN mkdir -p /home/kasm-user/.config/janus && \
    cat > /home/kasm-user/.config/janus/config.yaml <<EOF
organization: "Trial User"
default-directory: "/home/kasm-user/models"
execution-mode: "HERMES"
scheduler: "LOCAL"

hermes:
  container:
    docker-socket: "unix:///var/run/docker.sock"
    startup-timeout: "30s"
    cleanup: true
    image: "ghcr.io/shairozan/hermes:latest"

# Trial-specific: license on desktop
nonmem-license: "/home/kasm-user/Desktop/nonmem.lic"
EOF

# Create desktop shortcuts
RUN mkdir -p /home/kasm-user/Desktop && \
    cat > /home/kasm-user/Desktop/janus.desktop <<EOF
[Desktop Entry]
Type=Application
Name=Janus
Comment=Pharmacometric Modeling Interface
Exec=/usr/bin/janus
Icon=janus
Terminal=false
Categories=Science;
EOF

RUN cat > /home/kasm-user/Desktop/README.txt <<EOF
Welcome to Janus Trial Environment!

To get started:
1. Drag and drop your NONMEM license file (nonmem.lic) onto this desktop
2. Double-click the "Janus" icon to launch the application
3. Open a model from the pre-loaded examples in ~/models/acop/
4. Click "Execute" to run your first NONMEM model via Hermes

This trial session expires in 15 minutes.

Need help? Visit https://docs.janus.pharmalytica.com
EOF

RUN chmod +x /home/kasm-user/Desktop/janus.desktop && \
    chown -R kasm-user:kasm-user /home/kasm-user/.config && \
    chown -R kasm-user:kasm-user /home/kasm-user/Desktop

# Pre-pull Hermes image (so first execution is fast)
# This requires Docker socket at build time OR we pull at runtime
# For now, document that host should pre-pull: docker pull ghcr.io/shairozan/hermes:latest

USER kasm-user

# Set default working directory to models
WORKDIR /home/kasm-user/models

# KASM handles the rest (VNC server, noVNC, etc.)
```

### Trial Provisioning Service

**Technology**: Go web service with REST API

**Location**: `services/trial-provisioner/`

**Responsibilities**:
1. Handle trial request submissions
2. Create KASM workspaces via API
3. Track session lifecycle and enforce time limits
4. Clean up expired sessions
5. Collect usage analytics
6. Rate limiting and abuse prevention

**API Endpoints**:

```go
// POST /api/v1/trial/request
// Request a new trial session
type TrialRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Name     string `json:"name" validate:"required"`
    Company  string `json:"company"`
    UseCase  string `json:"use_case"`
}

type TrialResponse struct {
    SessionID   string    `json:"session_id"`
    WorkspaceURL string   `json:"workspace_url"`
    ExpiresAt   time.Time `json:"expires_at"`
    Instructions string   `json:"instructions"`
}

// GET /api/v1/trial/status/:session_id
// Check trial session status
type TrialStatus struct {
    SessionID   string    `json:"session_id"`
    State       string    `json:"state"` // "active", "expired", "terminated"
    ExpiresAt   time.Time `json:"expires_at"`
    RemainingSeconds int  `json:"remaining_seconds"`
}

// DELETE /api/v1/trial/terminate/:session_id
// User-initiated termination (optional)
type TerminateResponse struct {
    SessionID string `json:"session_id"`
    Message   string `json:"message"`
}
```

**Implementation** (`services/trial-provisioner/main.go`):

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type TrialProvisioner struct {
    kasmClient *KASMClient
    sessions   map[string]*TrialSession
}

type TrialSession struct {
    ID           string
    Email        string
    WorkspaceID  string
    WorkspaceURL string
    CreatedAt    time.Time
    ExpiresAt    time.Time
    State        string
}

type KASMClient struct {
    baseURL string
    apiKey  string
    apiSecret string
}

func (k *KASMClient) CreateWorkspace(ctx context.Context, sessionID string) (*KASMWorkspace, error) {
    // Call KASM API to create workspace
    // POST https://kasm.example.com/api/public/request_kasm
    // Returns workspace_id and URL

    payload := map[string]interface{}{
        "api_key": k.apiKey,
        "api_key_secret": k.apiSecret,
        "image_src": "pharmalytica/janus-trial:latest",
        "enable_sharing": false,
        "environment": map[string]string{
            "JANUS_TRIAL_SESSION": sessionID,
        },
    }

    // HTTP request to KASM API...
    // Return workspace details

    return &KASMWorkspace{
        ID:  "workspace-id-from-kasm",
        URL: "https://kasm.example.com/desktop/workspace-id",
    }, nil
}

func (k *KASMClient) DestroyWorkspace(ctx context.Context, workspaceID string) error {
    // Call KASM API to destroy workspace
    // POST https://kasm.example.com/api/public/destroy_kasm
    return nil
}

func (p *TrialProvisioner) HandleTrialRequest(c *gin.Context) {
    var req TrialRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    // Rate limiting check (by email/IP)
    if p.isRateLimited(req.Email, c.ClientIP()) {
        c.JSON(429, gin.H{"error": "too many requests"})
        return
    }

    // Create trial session
    sessionID := uuid.New().String()
    expiresAt := time.Now().Add(15 * time.Minute)

    workspace, err := p.kasmClient.CreateWorkspace(c.Request.Context(), sessionID)
    if err != nil {
        log.Printf("Failed to create KASM workspace: %v", err)
        c.JSON(500, gin.H{"error": "failed to provision trial"})
        return
    }

    session := &TrialSession{
        ID:           sessionID,
        Email:        req.Email,
        WorkspaceID:  workspace.ID,
        WorkspaceURL: workspace.URL,
        CreatedAt:    time.Now(),
        ExpiresAt:    expiresAt,
        State:        "active",
    }

    p.sessions[sessionID] = session

    // Schedule cleanup
    go p.scheduleCleanup(sessionID, 15*time.Minute)

    c.JSON(200, TrialResponse{
        SessionID:    sessionID,
        WorkspaceURL: workspace.URL,
        ExpiresAt:    expiresAt,
        Instructions: "Drag your NONMEM license file onto the desktop, then double-click Janus to start.",
    })

    // Send confirmation email (optional)
    go p.sendTrialEmail(req.Email, session)
}

func (p *TrialProvisioner) scheduleCleanup(sessionID string, duration time.Duration) {
    time.Sleep(duration)

    session, exists := p.sessions[sessionID]
    if !exists {
        return
    }

    log.Printf("Cleaning up expired trial session: %s", sessionID)

    if err := p.kasmClient.DestroyWorkspace(context.Background(), session.WorkspaceID); err != nil {
        log.Printf("Failed to destroy workspace %s: %v", session.WorkspaceID, err)
    }

    session.State = "expired"
}

func (p *TrialProvisioner) isRateLimited(email, ip string) bool {
    // Implement rate limiting logic
    // Example: max 1 trial per email per 24 hours
    // Example: max 5 trials per IP per hour
    return false
}

func (p *TrialProvisioner) sendTrialEmail(email string, session *TrialSession) {
    // Send email with trial link and instructions
    log.Printf("Sending trial email to %s for session %s", email, session.ID)
}

func main() {
    kasmClient := &KASMClient{
        baseURL:   "https://kasm.example.com",
        apiKey:    "your-api-key",
        apiSecret: "your-api-secret",
    }

    provisioner := &TrialProvisioner{
        kasmClient: kasmClient,
        sessions:   make(map[string]*TrialSession),
    }

    router := gin.Default()

    router.POST("/api/v1/trial/request", provisioner.HandleTrialRequest)
    router.GET("/api/v1/trial/status/:session_id", provisioner.HandleTrialStatus)
    router.DELETE("/api/v1/trial/terminate/:session_id", provisioner.HandleTrialTerminate)

    log.Println("Trial provisioner service starting on :8080")
    router.Run(":8080")
}
```

### Frontend Trial Request Page

**Location**: `services/trial-provisioner/web/index.html`

**Design**:
```html
<!DOCTYPE html>
<html>
<head>
    <title>Try Janus - Free 15-Minute Trial</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
        }
        .hero {
            text-align: center;
            margin-bottom: 40px;
        }
        .form-container {
            background: #f8f9fa;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .form-group {
            margin-bottom: 20px;
        }
        label {
            display: block;
            font-weight: 600;
            margin-bottom: 5px;
        }
        input, textarea {
            width: 100%;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 14px;
        }
        button {
            background: #007bff;
            color: white;
            padding: 12px 24px;
            border: none;
            border-radius: 4px;
            font-size: 16px;
            cursor: pointer;
        }
        button:hover {
            background: #0056b3;
        }
        .trial-info {
            background: #e7f3ff;
            border-left: 4px solid #007bff;
            padding: 15px;
            margin-top: 20px;
        }
        .success-message {
            background: #d4edda;
            border: 1px solid #c3e6cb;
            padding: 20px;
            border-radius: 4px;
            margin-top: 20px;
        }
        .countdown {
            font-size: 18px;
            font-weight: 600;
            color: #dc3545;
        }
    </style>
</head>
<body>
    <div class="hero">
        <h1>Try Janus in Your Browser</h1>
        <p>Experience Janus in a fully-configured environment. No installation required.</p>
    </div>

    <div class="form-container">
        <h2>Request Your Free Trial</h2>
        <form id="trialForm">
            <div class="form-group">
                <label for="name">Name *</label>
                <input type="text" id="name" required>
            </div>

            <div class="form-group">
                <label for="email">Email *</label>
                <input type="email" id="email" required>
            </div>

            <div class="form-group">
                <label for="company">Company/Organization</label>
                <input type="text" id="company">
            </div>

            <div class="form-group">
                <label for="useCase">What are you interested in? (optional)</label>
                <textarea id="useCase" rows="3"></textarea>
            </div>

            <button type="submit">Start My Trial</button>
        </form>

        <div class="trial-info">
            <strong>What you'll get:</strong>
            <ul>
                <li>✅ 15-minute browser-based desktop session</li>
                <li>✅ Janus pre-installed and configured</li>
                <li>✅ Hermes execution environment ready</li>
                <li>✅ Sample NONMEM models included</li>
                <li>✅ Just drag-and-drop your NONMEM license to start</li>
            </ul>
        </div>
    </div>

    <div id="successMessage" class="success-message" style="display: none;">
        <h3>🎉 Your Trial is Ready!</h3>
        <p>Click the button below to launch your Janus trial environment:</p>
        <a id="workspaceLink" href="#" target="_blank" style="
            display: inline-block;
            background: #28a745;
            color: white;
            padding: 15px 30px;
            text-decoration: none;
            border-radius: 4px;
            font-weight: 600;
            margin: 10px 0;
        ">Launch Janus Trial →</a>
        <p class="countdown">Time remaining: <span id="countdown">15:00</span></p>
        <p style="margin-top: 20px;">
            <strong>Quick Start:</strong><br>
            1. Drag your NONMEM license file onto the desktop<br>
            2. Double-click "Janus" to launch<br>
            3. Open a model from ~/models/acop/<br>
            4. Click "Execute" to run via Hermes
        </p>
    </div>

    <script>
        document.getElementById('trialForm').addEventListener('submit', async (e) => {
            e.preventDefault();

            const formData = {
                name: document.getElementById('name').value,
                email: document.getElementById('email').value,
                company: document.getElementById('company').value,
                use_case: document.getElementById('useCase').value,
            };

            try {
                const response = await fetch('/api/v1/trial/request', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(formData),
                });

                if (!response.ok) {
                    throw new Error('Failed to create trial');
                }

                const data = await response.json();

                // Hide form, show success
                document.querySelector('.form-container').style.display = 'none';
                document.getElementById('successMessage').style.display = 'block';

                // Set workspace link
                document.getElementById('workspaceLink').href = data.workspace_url;

                // Start countdown
                startCountdown(new Date(data.expires_at));

            } catch (error) {
                alert('Failed to create trial. Please try again later.');
                console.error(error);
            }
        });

        function startCountdown(expiresAt) {
            const countdownEl = document.getElementById('countdown');

            const interval = setInterval(() => {
                const now = new Date();
                const remaining = Math.max(0, expiresAt - now);

                if (remaining === 0) {
                    clearInterval(interval);
                    countdownEl.textContent = 'EXPIRED';
                    return;
                }

                const minutes = Math.floor(remaining / 60000);
                const seconds = Math.floor((remaining % 60000) / 1000);
                countdownEl.textContent = `${minutes}:${seconds.toString().padStart(2, '0')}`;
            }, 1000);
        }
    </script>
</body>
</html>
```

## Docker Socket Bind Mount Strategy

**Challenge**: Hermes needs Docker to execute models, but we don't want Docker-in-Docker complexity.

**Solution**: Bind mount host Docker socket into trial container.

**Host Configuration**:
```bash
# On KASM host, ensure Docker socket is accessible
chmod 666 /var/run/docker.sock  # Or use docker group

# Pre-pull Hermes image so trial containers can use it immediately
docker pull ghcr.io/shairozan/hermes:latest
```

**Container Configuration**:
```bash
# When creating trial container
docker run -d \
  -v /var/run/docker.sock:/var/run/docker.sock \
  pharmalytica/janus-trial:latest
```

**Security Implications**:
- Trial containers have full Docker access on host
- Can start/stop containers, access Docker images
- Requires trust in trial users OR container isolation (gVisor, Kata Containers)

**Mitigation Strategies**:
1. **Resource Limits**: Limit CPU/memory for trial containers
2. **Network Isolation**: Restrict network access from trial containers
3. **Image Whitelisting**: Only allow pulling from approved registries
4. **Execution Limits**: Hermes timeout prevents runaway executions
5. **Auto-Cleanup**: Trial containers destroyed after 15 minutes
6. **Monitoring**: Log all Docker API calls from trial containers

## License Handling

**User Experience**:
1. User receives KASM desktop URL
2. User opens URL in browser → sees Debian desktop
3. User drags `nonmem.lic` from local machine onto KASM desktop
4. KASM file transfer places file at `/home/kasm-user/Desktop/nonmem.lic`
5. Janus config pre-set to look for license at this path
6. User launches Janus → Hermes automatically uses license

**Janus Configuration** (in trial container):
```yaml
# /home/kasm-user/.config/janus/config.yaml
hermes:
  license-path: "/home/kasm-user/Desktop/nonmem.lic"
```

**License Security**:
- License file never leaves trial container
- Container destroyed after 15 minutes (license destroyed)
- No persistence between sessions
- User must re-upload license for each new trial

**License Validation** (optional enhancement):
```bash
# In trial container startup script
if [ -f /home/kasm-user/Desktop/nonmem.lic ]; then
    echo "NONMEM license detected - ready to execute models"
else
    echo "Please drag your NONMEM license file onto the desktop"
fi
```

## Sample Models

**Include Pre-Loaded Examples**:
- `/home/kasm-user/models/acop/` - ACOP example model
- `/home/kasm-user/models/simple/` - Simple one-compartment model
- `/home/kasm-user/models/README.md` - Instructions

**Benefits**:
- Users don't need to upload their own models
- Demonstrates Janus capabilities immediately
- Known-good models ensure successful execution

**Model Selection**:
- Use existing `testdata/acop` from Janus repo
- Small data files (fast execution)
- Well-documented control streams
- Demonstrate key Janus features

## Session Lifecycle Management

### Creation Flow
```
User submits trial request
    ↓
Validate request (email, rate limits)
    ↓
Generate session ID
    ↓
Call KASM API to create workspace
    ↓
Store session metadata
    ↓
Return workspace URL to user
    ↓
Schedule cleanup in 15 minutes
```

### Active Session Management
```bash
# Periodic cleanup job (runs every minute)
*/1 * * * * /usr/local/bin/trial-cleanup

# trial-cleanup script
#!/bin/bash
NOW=$(date +%s)

# Find expired trial containers
docker ps --filter "label=com.pharmalytica.trial.expires" --format "{{.ID}} {{.Label \"com.pharmalytica.trial.expires\"}}" | \
while read CONTAINER_ID EXPIRES; do
    if [ "$NOW" -gt "$EXPIRES" ]; then
        echo "Cleaning up expired trial container: $CONTAINER_ID"
        docker rm -f $CONTAINER_ID
    fi
done
```

### Cleanup Flow
```
15 minutes elapsed OR user terminates
    ↓
Call KASM API to destroy workspace
    ↓
Remove associated Docker volumes
    ↓
Update session state to "expired"
    ↓
Send follow-up email (optional)
    ↓
Log session analytics
```

## Analytics and Metrics

**Tracked Metrics**:
- Trial requests per day/week/month
- Successful provisioning rate
- Average session duration (full 15 min vs early termination)
- Models executed per session
- Geographic distribution (IP → location)
- Conversion rate (trial → signup/contact)

**Implementation**:
```go
type SessionAnalytics struct {
    SessionID       string
    Email           string
    Company         string
    CreatedAt       time.Time
    FirstAction     time.Time // First Janus launch
    ModelsExecuted  int
    ExecutionCount  int
    SessionDuration time.Duration
    IPAddress       string
    UserAgent       string
}

func (p *TrialProvisioner) recordAnalytics(sessionID string, event string) {
    // Store in database or send to analytics service
    analytics := p.getSessionAnalytics(sessionID)

    switch event {
    case "janus_launched":
        analytics.FirstAction = time.Now()
    case "model_executed":
        analytics.ExecutionCount++
    }

    p.saveAnalytics(analytics)
}
```

## Cost Estimation

### Infrastructure Costs (AWS Example)

**KASM Server**:
- EC2 instance: t3.xlarge (4 vCPU, 16GB RAM)
- Cost: ~$150/month
- Supports: ~10 concurrent trial sessions

**Storage**:
- EBS volumes: 100GB GP3
- Cost: ~$10/month

**Network**:
- Data transfer: ~100GB/month
- Cost: ~$10/month

**Total Monthly Cost**: ~$170/month for 10 concurrent users

**Per-Trial Cost**: Assuming 100 trials/month at 15 min each:
- ~$1.70 per trial

### Scaling Considerations

**Current Capacity** (single t3.xlarge):
- 10 concurrent sessions
- ~40 trials per hour (assuming 15-min sessions)
- ~960 trials per day (24/7 operation)

**Scaling Strategy**:
1. **Horizontal**: Add more KASM servers behind load balancer
2. **Vertical**: Upgrade to larger instances for more concurrency
3. **Auto-scaling**: Scale based on trial request volume

## Security Considerations

### Container Isolation

**Challenge**: Trial users have Docker socket access
**Risk**: Potential to escape container or interfere with other trials

**Mitigations**:
1. **Resource Limits**:
   ```bash
   docker run --cpus=2 --memory=4g --pids-limit=100 ...
   ```

2. **Network Isolation**:
   ```bash
   docker run --network=trial-network ...
   # trial-network has no internet egress (only to KASM)
   ```

3. **Read-Only Root FS** (where possible):
   ```bash
   docker run --read-only --tmpfs /tmp ...
   ```

4. **AppArmor/SELinux Profiles**: Restrict system calls

5. **Container Runtime Security**:
   - Consider gVisor for stronger isolation
   - Or Kata Containers for VM-level isolation

### Data Privacy

**User Data**:
- Email addresses stored for rate limiting and follow-up
- GDPR compliance: Allow users to request data deletion
- No persistent storage of models or results

**License Files**:
- Never stored outside ephemeral container
- Destroyed with container after 15 minutes
- No backups or logging of license content

### Rate Limiting

**Email-Based**:
- 1 trial per email per 24 hours
- Prevents single user from monopolizing resources

**IP-Based**:
- 5 trials per IP per hour
- Prevents abuse via multiple email addresses

**Implementation**:
```go
type RateLimiter struct {
    emailLimits map[string]time.Time // email → last trial time
    ipLimits    map[string][]time.Time // IP → trial times (sliding window)
}

func (r *RateLimiter) CheckEmail(email string) bool {
    lastTrial, exists := r.emailLimits[email]
    if !exists {
        return true // No previous trial
    }

    return time.Since(lastTrial) > 24*time.Hour
}

func (r *RateLimiter) CheckIP(ip string) bool {
    trials := r.ipLimits[ip]

    // Remove trials older than 1 hour
    cutoff := time.Now().Add(-1 * time.Hour)
    recent := []time.Time{}
    for _, t := range trials {
        if t.After(cutoff) {
            recent = append(recent, t)
        }
    }

    r.ipLimits[ip] = recent
    return len(recent) < 5
}
```

## Deployment Architecture

### Production Setup

```
Internet
    │
    ├─── CloudFlare (DDoS protection, CDN)
    │
    └─── Load Balancer (AWS ALB)
            │
            ├─── Trial Provisioner Service (ECS/Fargate)
            │    └─── Handles API requests
            │
            └─── KASM Workspaces Server (EC2)
                 └─── Runs trial containers
```

### DNS Configuration

```
try.janus.pharmalytica.com → Trial request page + API
kasm.janus.pharmalytica.com → KASM server (WebSocket VNC)
```

### SSL/TLS

- Let's Encrypt certificates for all domains
- Automatic renewal via certbot
- HTTPS required for WebSocket connections

## Implementation Plan

### Phase 1: KASM Container Development
**Goal**: Create working Janus trial container

- [ ] Create Dockerfile extending `kasmweb/debian-trixie-desktop`
- [ ] Install Janus from DEB package
- [ ] Install Docker CLI (no daemon)
- [ ] Pre-configure Janus for Hermes execution
- [ ] Add sample NONMEM models
- [ ] Create desktop shortcuts and README
- [ ] Test Docker socket bind mount access
- [ ] Test file drag-and-drop from browser to desktop
- [ ] Verify Hermes execution works with mounted socket

**Deliverable**: `pharmalytica/janus-trial:latest` container image

### Phase 2: Manual KASM Testing
**Goal**: Validate container works in KASM environment

- [ ] Set up KASM Workspaces server (dev environment)
- [ ] Manually create workspace with trial container
- [ ] Test full user workflow (drag license, execute model)
- [ ] Verify 15-minute lifecycle
- [ ] Test cleanup and resource reclamation
- [ ] Document any KASM-specific configurations needed

### Phase 3: Provisioner Service
**Goal**: Automate trial creation via API

- [ ] Implement Go service with Gin framework
- [ ] Integrate KASM API client
- [ ] Implement trial request endpoint
- [ ] Implement session lifecycle management
- [ ] Add rate limiting logic
- [ ] Add cleanup scheduler
- [ ] Implement analytics collection
- [ ] Add health check and monitoring endpoints

**Deliverable**: Trial provisioner service ready for deployment

### Phase 4: Frontend Development
**Goal**: User-facing trial request interface

- [ ] Design landing page UI/UX
- [ ] Implement trial request form
- [ ] Add form validation
- [ ] Implement countdown timer
- [ ] Add usage instructions
- [ ] Test across browsers (Chrome, Firefox, Safari)
- [ ] Add error handling and user feedback

**Deliverable**: Static website for trial requests

### Phase 5: Production Deployment
**Goal**: Deploy trial service to production

- [ ] Provision AWS infrastructure (EC2, ALB, etc.)
- [ ] Install and configure KASM Workspaces
- [ ] Deploy trial provisioner service
- [ ] Configure DNS and SSL certificates
- [ ] Set up monitoring and alerting
- [ ] Pre-pull Hermes images on host
- [ ] Configure automated backups (session data)
- [ ] Load testing and capacity planning
- [ ] Document runbooks for operations

### Phase 6: Analytics and Optimization
**Goal**: Monitor usage and improve experience

- [ ] Integrate analytics dashboard
- [ ] Set up conversion tracking
- [ ] A/B test trial duration (15 vs 20 vs 30 minutes)
- [ ] Collect user feedback via post-trial survey
- [ ] Optimize container startup time
- [ ] Analyze usage patterns and model common workflows

## Testing Strategy

### Manual Testing Checklist

**End-to-End User Flow**:
- [ ] Visit trial request page
- [ ] Submit trial request form
- [ ] Receive workspace URL
- [ ] Open workspace in browser
- [ ] See Debian desktop with Janus icon
- [ ] Drag NONMEM license file from local machine to desktop
- [ ] Verify license file appears on desktop
- [ ] Double-click Janus icon → application launches
- [ ] Open pre-loaded model from ~/models/acop/
- [ ] Click "Execute" → model runs via Hermes
- [ ] Verify execution completes successfully
- [ ] Check execution log and output files
- [ ] Wait for 15-minute expiry → session terminates

**Rate Limiting**:
- [ ] Submit 2 requests with same email → 2nd rejected
- [ ] Submit 6 requests from same IP → 6th rejected
- [ ] Wait 24 hours → email limit resets
- [ ] Wait 1 hour → IP limit resets

**Error Handling**:
- [ ] Submit invalid email → validation error
- [ ] KASM API failure → graceful error message
- [ ] Network interruption during session → reconnect works
- [ ] Browser refresh → session persists

### Automated Testing

**Unit Tests** (`services/trial-provisioner/`):
```go
func TestRateLimiter_Email(t *testing.T) {
    limiter := NewRateLimiter()

    // First request should succeed
    assert.True(t, limiter.CheckEmail("user@example.com"))
    limiter.RecordEmail("user@example.com")

    // Second request within 24h should fail
    assert.False(t, limiter.CheckEmail("user@example.com"))
}

func TestSessionLifecycle(t *testing.T) {
    provisioner := NewTrialProvisioner(mockKASMClient)

    // Create session
    session := provisioner.CreateSession("user@example.com")
    assert.NotEmpty(t, session.ID)
    assert.Equal(t, "active", session.State)

    // Simulate expiry
    provisioner.ExpireSession(session.ID)
    assert.Equal(t, "expired", session.State)
}
```

**Integration Tests**:
```bash
#!/bin/bash
# Test full trial provisioning flow

# Start provisioner service
./trial-provisioner &
PID=$!

# Submit trial request
RESPONSE=$(curl -X POST http://localhost:8080/api/v1/trial/request \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","name":"Test User"}')

SESSION_ID=$(echo $RESPONSE | jq -r '.session_id')
WORKSPACE_URL=$(echo $RESPONSE | jq -r '.workspace_url')

# Verify session created
if [ -n "$SESSION_ID" ]; then
    echo "✅ Session created: $SESSION_ID"
else
    echo "❌ Session creation failed"
    exit 1
fi

# Check status endpoint
STATUS=$(curl http://localhost:8080/api/v1/trial/status/$SESSION_ID)
STATE=$(echo $STATUS | jq -r '.state')

if [ "$STATE" = "active" ]; then
    echo "✅ Session is active"
else
    echo "❌ Session state incorrect: $STATE"
    exit 1
fi

# Cleanup
kill $PID
```

## Open Questions

1. **Should we require CAPTCHA for trial requests?**
   - **Decision**: Yes, to prevent bot abuse. Use hCaptcha or reCAPTCHA.

2. **Should we send confirmation emails before provisioning?**
   - **Decision**: No, provision immediately. Email is for follow-up only.

3. **What happens if user requests multiple trials?**
   - **Decision**: Rate limit to 1 per email per 24 hours.

4. **Should we allow extending the 15-minute session?**
   - **Decision**: No extensions for MVP. Keeps costs predictable.

5. **How do we handle peak demand (conference demos)?**
   - **Decision**: Auto-scaling or temporary capacity increase for events.

6. **Should we support file upload in addition to drag-and-drop?**
   - **Decision**: Drag-and-drop only for MVP (native KASM feature).

7. **What if user's NONMEM license doesn't work?**
   - **Decision**: Provide troubleshooting instructions in README. No support for MVP.

8. **Should we offer paid extended trials?**
   - **Decision**: Post-MVP. Free 15-minute trials only initially.

## Future Enhancements

1. **Extended Trial Tiers**:
   - Free: 15 minutes
   - Email verification: 30 minutes
   - Paid: 4-hour session ($10)

2. **Persistent Workspaces**:
   - Allow users to save session state
   - Resume later with same configuration

3. **Collaboration Features**:
   - Share trial session with colleague
   - Co-browsing for live demos

4. **Recording and Playback**:
   - Record trial session for later review
   - Automated demo videos

5. **Custom Model Upload**:
   - Allow users to upload their own models
   - Automatic data file detection

6. **Integration with CRM**:
   - Sync trial requests to Salesforce/HubSpot
   - Automated lead nurturing

7. **White-Label Trials**:
   - Branded trial environments for partners
   - Custom configurations per organization

8. **Offline Trial Mode**:
   - Downloadable VM image for offline evaluation
   - Pre-configured VirtualBox/VMware image

9. **API Access**:
   - Programmatic trial creation for integrations
   - Embed trial widget on partner websites

10. **Advanced Analytics**:
    - Heatmaps of UI interaction
    - Feature usage tracking
    - A/B testing framework

## Success Metrics

**Adoption Metrics**:
- Trial requests per month
- Successful trial completion rate (>80% target)
- Average session duration (target: 12+ minutes of 15)
- Models executed per session (target: 2+)

**Conversion Metrics**:
- Trial → demo request conversion (target: 20%)
- Trial → paid customer conversion (target: 5%)
- Time from trial to first purchase

**Technical Metrics**:
- Provisioning success rate (target: >99%)
- Average provisioning time (target: <30 seconds)
- Container uptime during session (target: >99%)
- Resource utilization (CPU/memory)

**User Satisfaction**:
- Post-trial survey rating (target: 4+ stars)
- Support ticket volume
- Feature request frequency

## Cost-Benefit Analysis

### Costs

**Infrastructure**: ~$170/month (100 trials/month)
**Development**: ~80 hours (initial implementation)
**Maintenance**: ~5 hours/month (monitoring, updates)

### Benefits

**Reduced Sales Friction**:
- Current: 4-hour setup, ~30% completion rate
- Trial: 2-minute setup, ~90% completion rate
- **3x increase in qualified leads**

**Faster Sales Cycle**:
- Current: 2-week evaluation period
- Trial: Instant evaluation → demo → purchase
- **50% reduction in sales cycle length**

**Lower Support Burden**:
- Eliminate installation support tickets
- Standardized environment (no "works on my machine")
- **30% reduction in pre-sales support time**

**Marketing Value**:
- Demo material for conferences
- Social proof (usage statistics)
- Lead generation tool

**ROI Estimate**:
- If 5% of trials convert to $50k annual contracts
- 100 trials/month → 5 customers/month → $250k/year revenue
- Infrastructure cost: $2k/year
- **ROI: 12,500%**

## References

- KASM Workspaces Documentation: https://kasmweb.com/docs
- KASM API Reference: https://kasmweb.com/docs/latest/api/index.html
- Docker Socket Security: https://docs.docker.com/engine/security/
- Gin Web Framework: https://gin-gonic.com/
- Hermes Executor: [internal/execution/hermes.go](../../internal/execution/hermes.go)
- Janus Configuration: [internal/config/config.go](../../internal/config/config.go)
