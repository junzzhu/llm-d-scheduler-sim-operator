# Configuration

## Sample Manifests

Use the curated samples in `config/samples`:
- `config/samples/sim_v1alpha1_simulatordeployment_minimal.yaml`
- `config/samples/sim_v1alpha1_simulatordeployment_full.yaml`
- `config/samples/sim_v1alpha1_simulatordeployment_istio.yaml`
- `config/samples/sim_v1alpha1_schedulerinstall.yaml`

## SimulatorDeploymentSpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `replicas` | int32 | 2 | Number of simulator pods (deprecated, use prefill/decode) |
| `image` | string | `ghcr.io/llm-d/llm-d-simulator:latest` | Container image |
| `logVerbosity` | int32 | 5 | klog verbosity for simulator pods |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |
| `service` | ServiceConfig | - | Service configuration |
| `gateway` | GatewayConfig | - | Gateway configuration (legacy) |
| `loadBalancing` | LoadBalancingConfig | - | Load balancing settings |
| `epp` | EPPConfig | - | EPP (Endpoint Picker) configuration |
| `inferenceGateway` | InferenceGatewayConfig | - | Inference gateway configuration |
| `prefill` | StageConfig | - | Prefill stage configuration |
| `decode` | StageConfig | - | Decode stage configuration |

## EPPConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable EPP deployment |
| `replicas` | int32 | 1 | Number of EPP pods |
| `image` | string | `ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0` | EPP container image |
| `port` | int32 | 8100 | EPP service port |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |

## InferenceGatewayConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable inference gateways |
| `standard` | GatewayInstanceConfig | - | Standard gateway (kgateway) configuration |
| `istio` | GatewayInstanceConfig | - | Istio gateway configuration |

## GatewayInstanceConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable this gateway instance |
| `replicas` | int32 | 1 | Number of gateway pods |
| `image` | string | varies | Gateway image (kgateway or istio) |
| `port` | int32 | 8080 | Gateway service port |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |

## StageConfig (Prefill/Decode)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable this stage |
| `replicas` | int32 | 2 | Number of pods for this stage |
| `image` | string | `docker.io/library/llm-d-simulator:local` | Container image |
| `port` | int32 | 8200 | Service port |
| `logVerbosity` | int32 | 5 | klog verbosity for this stage |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |
| `args` | []string | - | Additional container arguments |

## ServiceConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `gaie-inference-scheduling-proxy` | Service name |
| `port` | int32 | 8200 | Service port |
| `type` | string | `ClusterIP` | Service type |

## LoadBalancingConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable custom load balancing |
| `algorithm` | string | `ROUND_ROBIN` | ROUND_ROBIN, LEAST_REQUEST, RANDOM, LEAST_CONN |
| `connectionPool` | ConnectionPoolConfig | - | Connection pool settings |

## Critical Port Map

| Component | Port Type | Port | Description |
|-----------|-----------|------|-------------|
| **Gateway** | Service Port | 8080 | External port exposed by the Service |
| **Gateway** | Target Port | 80 | Port the Gateway Pod listens on |
| **Gateway** | Admin/Probe | 19000 | Envoy Admin interface |
| **Backend** | Service Port | 8200 | Simulator backend port |
| **EPP** | Service Port | 8100 | Endpoint Picker port |
| **EPP** | Health Port | 9003 | EPP liveness/readiness |
