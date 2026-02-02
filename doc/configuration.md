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
| `verbosity` | int32 | 1 | EPP log verbosity (maps to `--v`) |
| `args` | []string | - | Additional EPP container arguments |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |

Note: The EPP gRPC server listens on the configured `port`. Ensure the Service
port matches the gRPC port you expect Envoy/ext_proc to connect to.

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

## SchedulerInstallSpec

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `schedulerNamespace` | string | CR namespace | Namespace for scheduler components |
| `simulatorNamespace` | string | - | Namespace for simulator backends |
| `proxyService` | ProxyServiceConfig | - | Proxy Service configuration |
| `epp` | SchedulerEPPConfig | - | EPP configuration |
| `gateway` | SchedulerGatewayConfig | - | Gateway configuration |
| `routing` | SchedulerRoutingConfig | - | HTTPRoute + ReferenceGrant configuration |
| `destinationRule` | LoadBalancingConfig | - | Istio DestinationRule configuration |
| `envoyFilter` | SchedulerEnvoyFilterConfig | - | EnvoyFilter ext_proc configuration |

## SchedulerRoutingConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable HTTPRoute + ReferenceGrant |
| `backendType` | string | `Service` | Backend type: `Service` or `InferencePool` |
| `inferencePool` | InferencePoolRef | - | InferencePool reference when `backendType=InferencePool` |
| `httpRouteName` | string | `llm-d-inference-scheduling` | HTTPRoute name |
| `parentGateway` | GatewayRef | - | Parent Gateway reference |

## SchedulerEPPConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable EPP deployment |
| `name` | string | `gaie-inference-scheduling-epp` | EPP deployment/service name |
| `replicas` | int32 | 1 | Number of EPP pods |
| `image` | string | `ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0` | EPP container image |
| `port` | int32 | 9002 | EPP service port |
| `verbosity` | int32 | 1 | EPP log verbosity (maps to `--v`) |
| `args` | []string | - | Additional EPP container arguments |
| `poolName` | string | `gaie-inference-scheduling` | InferencePool name watched by EPP |
| `poolNamespace` | string | simulatorNamespace | InferencePool namespace |
| `resources` | ResourceRequirements | - | CPU/memory requests and limits |

Note: The EPP gRPC server listens on the configured `port`. Ensure the Service
port matches the gRPC port you expect Envoy/ext_proc to connect to.

## InferencePoolRef

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | InferencePool name |
| `namespace` | string | simulatorNamespace | InferencePool namespace |
| `port` | int32 | - | Port for backendRef (if required) |

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

## Simulator Metrics

The simulator exposes Prometheus metrics on the HTTP port (default 8200) at `/metrics`.

### Option A: Port-forward the proxy Service

```bash
kubectl port-forward -n ${NS_SIM} svc/gaie-inference-scheduling-proxy 18200:8200
curl http://localhost:18200/metrics
```

Expected output (example):
```
vllm:time_per_output_token_seconds_bucket{model_name="random",le="+Inf"} 15
vllm:time_per_output_token_seconds_sum{model_name="random"} 0
vllm:time_per_output_token_seconds_count{model_name="random"} 15
# HELP vllm:time_to_first_token_seconds Histogram of time to first token in seconds.
# TYPE vllm:time_to_first_token_seconds histogram
vllm:time_to_first_token_seconds_bucket{model_name="random",le="0.001"} 4
vllm:time_to_first_token_seconds_bucket{model_name="random",le="0.005"} 4
```

### Option B: Port-forward a specific decode pod

```bash
kubectl get pods -n ${NS_SIM} -l llm-d.ai/role=decode
kubectl port-forward -n ${NS_SIM} pod/<decode-pod> 18200:8200
curl http://localhost:18200/metrics
```

If `/metrics` returns 404, check simulator logs to confirm metrics are enabled for the image and configuration in use.
