# LLM-D Scheduler-Sim Operator

A minimal Kubernetes operator that simplifies deployment and management of llm-d inference simulators, replacing manual deployment steps with declarative configuration.

## Overview

This operator automates the deployment of llm-d simulator instances as documented in the [llm-d-scheduler-sim](https://github.com/llm-d/llm-d-scheduler-sim) project. It replaces manual `kubectl scale`, service configuration, and Istio DestinationRule creation with a single Custom Resource.

## Features

- **Full LLM-D Stack Deployment**: Deploy complete llm-d inference architecture
- **EPP (Endpoint Picker)**: Automated deployment of endpoint picker service
- **Inference Gateways**: Support for both standard (kgateway) and Istio gateways
- **Separate Prefill/Decode Stages**: Independent scaling and configuration for prefill and decode
- **Automated Deployment**: Creates simulator deployments with proper labels
- **Service Management**: Automatically creates services for all components
- **Scaling**: Simple replica management through CR spec
- **Load Balancing**: Optional Istio DestinationRule configuration
- **Status Reporting**: Real-time status of replicas and endpoints
- **Backward Compatible**: Supports legacy single-deployment mode

## Manual Steps Replaced

### Before (Manual)
```bash
# Scale deployment
kubectl scale deployment ms-sim-llm-d-modelservice-decode -n llm-d-sim --replicas=2

# Verify pods
kubectl get pods -n llm-d-sim -l llm-d.ai/role=decode

# Check endpoints
kubectl get endpoints gaie-inference-scheduling-proxy -n llm-d-sim

# Create DestinationRule for load balancing
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: gaie-inference-scheduling-proxy-lb
  namespace: llm-d-sim
spec:
  host: gaie-inference-scheduling-proxy.llm-d-sim.svc.cluster.local
  trafficPolicy:
    loadBalancer:
      simple: ROUND_ROBIN
EOF
```

### After (With Operator)
```yaml
apiVersion: sim.llm-d.io/v1alpha1
kind: SimulatorDeployment
metadata:
  name: llm-sim
  namespace: llm-d-sim
spec:
  replicas: 2
  loadBalancing:
    enabled: true
    algorithm: "ROUND_ROBIN"
```

## Installation

### Prerequisites

- Kubernetes cluster v1.20+
- kubectl configured
- Go 1.22+ (for building from source)

### Install CRDs

```bash
kubectl apply -f config/crd/sim.llm-d.io_simulatordeployments.yaml
```

### Run Operator

**Prerequisite: Set up RBAC**
The EPP component requires specific permissions to list pods and inference resources.
```bash
# Create ServiceAccount and RoleBinding
./hack/create-epp-rbac.sh
```

**Deploy to cluster (Recommended)**
Use the provided script to build and deploy with all necessary configurations (args, env vars, ConfigMaps):
```bash
# Build and deploy operator
./hack/redeploy-with-fixes.sh
```

## Quick Start

For a detailed, step-by-step guide to deploying the full stack, see [HANDS-ON.md](HANDS-ON.md).

### Minimal Deployment

```bash
# Create namespace
kubectl create namespace llm-d-sim

# Deploy minimal CR
kubectl apply -f config/samples/sim_v1alpha1_simulatordeployment_minimal.yaml
```

To verify:
```bash
kubectl get simulatordeployment -n llm-d-sim -w
```

## Sample Configurations

### Minimal Configuration

```yaml
apiVersion: sim.llm-d.io/v1alpha1
kind: SimulatorDeployment
metadata:
  name: llm-sim-minimal
  namespace: llm-d-sim
spec:
  replicas: 2
```

### With Istio Load Balancing

```yaml
apiVersion: sim.llm-d.io/v1alpha1
kind: SimulatorDeployment
metadata:
  name: llm-sim-istio
  namespace: llm-d-sim
spec:
  replicas: 2
  service:
    name: "gaie-inference-scheduling-proxy"
    port: 8200
  loadBalancing:

### Full LLM-D Architecture

Deploy the complete llm-d stack with EPP, gateways, and separate prefill/decode stages:

```yaml
apiVersion: sim.llm-d.io/v1alpha1
kind: SimulatorDeployment
metadata:
  name: llm-sim-full
  namespace: llm-d-sim
spec:
  # EPP (Endpoint Picker)
  epp:
    enabled: true
    replicas: 1
    image: "ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0"
    port: 8100
  
  # Inference Gateways
  inferenceGateway:
    enabled: true
    standard:
      enabled: true
      replicas: 1
      image: "cr.kgateway.dev/kgateway-dev/envoy-wrapper:v2.1.1"
    istio:
      enabled: true
      replicas: 1
      image: "docker.io/istio/proxyv2:1.28.1"
  
  # Prefill Stage
  prefill:
    enabled: true
    replicas: 2
    image: "docker.io/library/llm-d-simulator:local"
  
  # Decode Stage
  decode:
    enabled: true
    replicas: 2
    image: "docker.io/library/llm-d-simulator:local"
  
  loadBalancing:
    enabled: true
    algorithm: "ROUND_ROBIN"
```

This creates the following deployments:
- `gaie-sim-epp` - Endpoint picker service
- `infra-sim-inference-gateway` - Standard gateway (kgateway)
- `infra-sim-inference-gateway-istio` - Istio gateway
- `ms-sim-llm-d-modelservice-prefill` - Prefill stage pods
- `ms-sim-llm-d-modelservice-decode` - Decode stage pods

### Prefill/Decode Separation

Deploy with separate prefill and decode stages for independent scaling:

```yaml
apiVersion: sim.llm-d.io/v1alpha1
kind: SimulatorDeployment
metadata:
  name: llm-sim-prefill-decode
  namespace: llm-d-sim
spec:
  prefill:
    enabled: true
    replicas: 2
    resources:
      requests:
        cpu: "100m"
        memory: "256Mi"
  
  decode:
    enabled: true
    replicas: 4  # Scale decode independently
    resources:
      requests:
        cpu: "100m"
        memory: "256Mi"
```

## Configuration Reference

### SimulatorDeploymentSpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `replicas` | int32 | 2 | Number of simulator pods (deprecated, use prefill/decode) |
| `image` | string | `ghcr.io/llm-d/llm-d-simulator:latest` | Container image |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |
| `service` | ServiceConfig | - | Service configuration |
| `gateway` | GatewayConfig | - | Gateway configuration (legacy) |
| `loadBalancing` | LoadBalancingConfig | - | Load balancing settings |
| `epp` | EPPConfig | - | EPP (Endpoint Picker) configuration |
| `inferenceGateway` | InferenceGatewayConfig | - | Inference gateway configuration |
| `prefill` | StageConfig | - | Prefill stage configuration |
| `decode` | StageConfig | - | Decode stage configuration |

### EPPConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable EPP deployment |
| `replicas` | int32 | 1 | Number of EPP pods |
| `image` | string | `ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0` | EPP container image |
| `port` | int32 | 8100 | EPP service port |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |

### InferenceGatewayConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable inference gateways |
| `standard` | GatewayInstanceConfig | - | Standard gateway (kgateway) configuration |
| `istio` | GatewayInstanceConfig | - | Istio gateway configuration. **Note: Enabling this without Istio installed will cause the Istio gateway pod to crash. This is harmless if you are using the Standard gateway.** |

### GatewayInstanceConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable this gateway instance |
| `replicas` | int32 | 1 | Number of gateway pods |
| `image` | string | varies | Gateway image (kgateway or istio) |
| `port` | int32 | 8080 | Gateway service port |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |

### StageConfig (Prefill/Decode)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable this stage |
| `replicas` | int32 | 2 | Number of pods for this stage |
| `image` | string | `docker.io/library/llm-d-simulator:local` | Container image |
| `port` | int32 | 8200 | Service port |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |
| `args` | []string | - | Additional container arguments |

### ServiceConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `gaie-inference-scheduling-proxy` | Service name |
| `port` | int32 | 8200 | Service port |
| `type` | string | `ClusterIP` | Service type |

### LoadBalancingConfig

| `enabled` | bool | false | Enable custom load balancing |
| `algorithm` | string | `ROUND_ROBIN` | Algorithm: ROUND_ROBIN, LEAST_REQUEST, RANDOM, LEAST_CONN |
| `connectionPool` | ConnectionPoolConfig | - | Connection pool settings |

### Critical Port Map

Correct port configuration is essential for the system to work.

| Component | Port Type | Port | Description |
|-----------|-----------|------|-------------|
| **Gateway** | Service Port | `8080` | External port exposed by the Service. Use this for `kubectl port-forward`. |
| **Gateway** | Target Port | `80` | The port the Gateway Pod actually listens on for traffic. |
| **Gateway** | Admin/Probe | `19000` | Envoy Admin interface, used for Liveness/Readiness probes. |
| **Backend** | Service Port | `8200` | Port used by the Simulator/ModelService backend. |
| **EPP** | Service Port | `8100` | Port exposed by the Endpoint Picker. |
| **EPP** | Health Port | `9003` | TCP port used for EPP liveness/readiness probes. |

## Operations

### Scaling

```bash
# Scale to 4 replicas
kubectl patch simulatordeployment llm-sim-minimal -n llm-d-sim \
  -p '{"spec":{"replicas":4}}' --type=merge

# Or edit directly
kubectl edit simulatordeployment llm-sim-minimal -n llm-d-sim
```

### Updating Configuration

```bash
# Enable load balancing
kubectl patch simulatordeployment llm-sim-minimal -n llm-d-sim \
  --type=merge -p '
{
  "spec": {
    "loadBalancing": {
      "enabled": true,
      "algorithm": "LEAST_REQUEST"
    }
  }
}'
```

### Checking Status

```bash
# Get status
kubectl get simulatordeployment -n llm-d-sim

# Example output:
# NAME               REPLICAS   READY   AGE
# llm-sim-minimal    2          2       5m

# Detailed status
kubectl get simulatordeployment llm-sim-minimal -n llm-d-sim -o yaml
```

## Integration with llm-d-scheduler

The operator creates resources compatible with the llm-d scheduler setup:

1. **Deployment**: Named `ms-sim-{name}-decode` with label `llm-d.ai/role=decode`
2. **Service**: Named `gaie-inference-scheduling-proxy` (configurable)
3. **Endpoints**: Automatically populated from deployment pods

### Which path is used by the current settings?

With the current operator defaults and samples:

- **Scheduler path is used**: `Gateway (Istio class) -> HTTPRoute -> proxy Service -> simulator pods`
- The Gateway data plane is Istio, but the **routing path is still the scheduler path** because requests go through the scheduler gateway and HTTPRoute.

Direct service path is only used if you bypass the scheduler gateway and call the simulator proxy Service directly.

#### Request flow (current default)
```
Client
  -> Scheduler Gateway (Istio data plane)
  -> HTTPRoute
  -> gaie-inference-scheduling-proxy (Service)
  -> simulator decode pods
```

#### Direct service path (bypass scheduler)
```
Client
  -> gaie-inference-scheduling-proxy (Service)
  -> simulator decode pods
```

Example commands:
```bash
# Scheduler path (recommended): port-forward scheduler gateway service
kubectl port-forward -n llm-d-inference-scheduler \
  svc/infra-inference-scheduling-inference-gateway-istio 8080:80
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"random","prompt":"test","max_tokens":5}'

# Direct service path (bypass scheduler)
kubectl port-forward -n llm-d-sim svc/gaie-inference-scheduling-proxy 8200:8200
curl -X POST http://localhost:8200/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"random","prompt":"test","max_tokens":5}'
```

### Is round-robin used with two decode pods?

It depends on which layer is making the routing decision:

- **Scheduler path (current default)**: EPP chooses the backend based on scorers (load, KV-cache, etc.), so it is **not round-robin** by default and can be intentionally uneven.
- **Direct service path**: Kubernetes Service load-balancing is used (roughly round-robin **per connection**).
- **Istio balancing**: Istio’s default is `LEAST_REQUEST`. You only get round-robin if a `DestinationRule` explicitly sets `simple: ROUND_ROBIN`.

Check the active Istio policy (if present):
```bash
kubectl get destinationrule gaie-inference-scheduling-proxy-lb -n llm-d-sim \
  -o jsonpath='{.spec.trafficPolicy.loadBalancer}'
```

### Does scoring work with Istio + scheduler path?

Yes, **if the Gateway is configured to call EPP via ext_proc** (Gateway API Inference Extension or equivalent filter configuration).
This operator deploys EPP and the scheduler Gateway/HTTPRoute, but **it does not inject ext_proc configuration by itself**.
If your Gateway implementation is already configured for EPP (e.g., via GatewayParameters/EnvoyFilter), scoring will work.
If not, requests will still be routed, but EPP scoring will not be applied.

#### GatewayParameters vs EnvoyFilter (what they do and what this operator manages)

**GatewayParameters (kgateway-specific)**:
- Used with the **kgateway** GatewayClass.
- Attaches config to the Gateway/Service at the **Gateway API controller layer**.
- Typical use: define Envoy bootstrap, extensions, or ext_proc integration in a controller-native way.
- In this repo, the operator **does not** create GatewayParameters.

**EnvoyFilter (Istio-specific)**:
- Used with the **istio** GatewayClass.
- Injects Envoy filters (e.g., `ext_proc`) into the Gateway proxy **after** it is created.
- Typical use: configure EPP integration by pointing ext_proc to the EPP service.
- In this repo, the operator **does not** create EnvoyFilters.

#### What this means for current settings

- **Current defaults use Istio** (`gateway.className: istio`), so you need an **EnvoyFilter** (or equivalent Istio GatewayParameters if available) to enable EPP scoring.
- The operator **does deploy EPP**, so once the ext_proc filter is installed, scoring is feasible without changing this operator.
- If you switch to **kgateway**, you would use **GatewayParameters** instead of EnvoyFilter for the ext_proc wiring.

Practical takeaway: **routing works now**, and **scoring works once the Gateway is configured to call EPP**.

### Bypass setup and impact on KV-scoring tests

This project uses a **bypass** route in the scheduler integration to avoid InferencePool headless service/port issues:
`HTTPRoute -> Service (gaie-inference-scheduling-proxy) -> simulator pods`

This makes routing reliable, but it **bypasses InferencePool-based selection**, so:

- **KV-scoring tests are not meaningful** when the bypass path is used.
- EPP may be running, but it is **not influencing backend selection** unless the Gateway is configured with ext_proc and the request path is mediated by InferencePool/EPP.

If you want KV-scoring behavior to show up in tests:
1) Enable ext_proc on the Gateway (EnvoyFilter), and
2) Route through an InferencePool backendRef instead of the proxy Service.

### Testing with Scheduler

```bash
# Set environment variables
export NAMESPACE_IS=llm-d-inference-scheduler
export NAMESPACE_SIM=llm-d-sim

# Get scheduler route
SCHED_ROUTE_URL=$(oc get route llm-sched -n ${NAMESPACE_IS} -o jsonpath='{.spec.host}')

# Send test requests
for i in {1..8}; do
  curl -X POST http://${SCHED_ROUTE_URL}/v1/completions \
    -H "Content-Type: application/json" \
    -d '{"model": "random", "prompt": "test", "max_tokens": 5}'
  sleep 0.5
done

# Check distribution in gateway logs
kubectl logs -n ${NAMESPACE_IS} \
  -l app.kubernetes.io/gateway=infra-inference-scheduling-inference-gateway -f
```

## Troubleshooting

### Pods CrashLoopBackOff

1.  **Check Probes**:
    -   **Gateway**: Ensure readiness/liveness probes use port `19000` (Admin interface), NOT `8082` or `8080`.
    -   **EPP**: Ensure probes use TCP socket on port `9003`. gRPC probes may fail if the service name doesn't match.

2.  **Check Permissions (RBAC)**:
    -   If EPP crashes with "forbidden" errors, run `./hack/create-epp-rbac.sh` to fix ServiceAccount permissions.

3.  **Check Logs**:
    ```bash
    kubectl logs -n llm-d-sim -l component=epp
    kubectl logs -n llm-d-sim -l component=gateway
    ```

### Service Endpoints Not Populated / Connection Refused

1.  **Check Service TargetPort**:
    -   The Gateway Service must target port `80` (where the pod listens), not `8080`.
    -   Verify with: `kubectl get svc infra-sim-inference-gateway -n llm-d-sim -o yaml`

2.  **Check Backend Configuration**:
    -   Ensure Gateway ConfigMap points to the correct backend service (e.g., `ms-sim-llm-d-modelservice-decode`) on port `8200`.
    -   "No healthy upstream" usually means the Envoy config points to a wrong service or port.

### Load Balancing Not Working

```bash
# Check if DestinationRule was created (requires Istio)
kubectl get destinationrule -n llm-d-sim

# Verify Istio is installed
kubectl get pods -n istio-system
```

## Development

### Building

```bash
# Build binary
go build -o bin/manager main.go

# Build container image
docker build -t llm-d-scheduler-sim-operator:latest .
```

### Generate CRDs

```bash
# Install controller-gen
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Generate CRDs
controller-gen crd paths="./api/..." output:crd:dir=./config/crd
```

## Comparison with Manual Approach

| Aspect | Manual | With Operator |
|--------|--------|---------------|
| Deployment | Multiple kubectl commands | Single CR apply |
| Scaling | `kubectl scale` | Update `replicas` in CR |
| Load Balancing | Manual DestinationRule | Set `loadBalancing.enabled: true` |
| Status Checking | Multiple kubectl get commands | `kubectl get simulatordeployment` |
| Configuration | Multiple YAML files | Single CR |
| GitOps | Difficult to track | Declarative, version-controlled |


## Future Work

### EnvoyFilter example (Istio ext_proc -> EPP)

Apply this in `llm-d-inference-scheduler` to enable scoring. It targets the scheduler gateway pods and points ext_proc to the EPP service.

Sample manifest:
`config/samples/envoyfilter-epp.yaml`

Apply the filter:
```bash
kubectl apply -f config/samples/envoyfilter-epp.yaml
```

Quick validation:
```bash
# Confirm EnvoyFilter is attached
kubectl get envoyfilter -n llm-d-inference-scheduler

# Check gateway has the ext_proc filter
istioctl proxy-config listeners -n llm-d-inference-scheduler \
  $(kubectl get pod -n llm-d-inference-scheduler \
    -l gateway.networking.k8s.io/gateway-name=infra-inference-scheduling-inference-gateway \
    -o jsonpath='{.items[0].metadata.name}') | rg ext_proc

# Check EPP logs for incoming ext_proc requests
kubectl logs -n llm-d-inference-scheduler \
  deploy/gaie-inference-scheduling-epp --tail=100
```

## License

Apache License 2.0
