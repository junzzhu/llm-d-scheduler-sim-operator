# LLM-D Scheduler-Sim Operator

A minimal Kubernetes operator that simplifies deployment and management of llm-d inference scheduler and simulator, with EPP-enabled routing for scoring-aware backend selection.

## Overview

This operator automates both llm-d **simulator** and **scheduler** setup as documented in [llm-d-scheduler-sim](https://github.com/llm-d/llm-d-scheduler-sim) and [llm-d-inference-scheduler](https://github.com/llm-d/llm-d-inference-scheduler) projects. It replaces manual simulator scaling, service wiring, Gateway API routing, and Istio load-balancing with declarative Custom Resources (`SimulatorDeployment` and `SchedulerInstall`).

Current workflow supports EPP (ext_proc) scoring on the scheduler gateway, enabling load/prefix/KV-based routing to prefill/decode pods.

Key capabilities:
- **LLM-D stack deployment** (scheduler + simulator)
- **EPP (Endpoint Picker)** deployment and integration
- **Gateway API routing** (Istio default; standard gateways supported)
- **Separate prefill/decode stages** with independent scaling
- **Service management + scaling** via CR specs

## EPP-enabled Workflow

- **EPP-enabled Workflow:** Client → Scheduler Gateway → ext_proc (EPP) → HTTPRoute → InferencePool → Simulator decode pods
- **CRDs:** `SchedulerInstall` for scheduler path, `SimulatorDeployment` for simulator components.
- **Gateway:** Istio (Gateway API HTTPRoute) is the default data plane.
 - **Instrumented EPP builds:** For the live-debug workflow and dev image setup, see `epp-dev/README.md`.
- **Fallback path:** Direct access via `gaie-inference-scheduling-proxy` bypasses EPP/scoring.

## KV Cache-Aware Scoring (ext_proc + EPP)

Implemented end-to-end routing via **EPP (ext_proc) + InferencePool** so the gateway can pick
backends based on scoring (load, prefix cache, KV cache). This is the foundation for cache-aware scheduling
and reduced TTFT in real deployments.

Why this is implemented:
- InferencePool gives the gateway a stable backend object for scheduling.
- EPP computes per-pod scores (load/prefix/KV) and returns the best target.
- ext_proc attaches EPP into the request path without changing client behavior.

Note: Scoring depends on the scheduler path using InferencePool state. If routing bypasses the scheduler
or InferencePool is not populated, EPP may return default/neutral scores and routing will still follow the
proxy Service selector.

### Scoring Test Results (Prefix Cache Skew)

#### Minimal Test Design

Use the minimal script `script/kv-prefix-cache-test.sh`:
- Request 1 goes through the gateway with a long prompt (cold cache).
- Requests 2+ repeat the same prompt through the gateway.
- Verify EPP scoring logs and routing selections to 3 instances of disaggregated P/D pods.

Note: restart the EPP pod before the test to clear any stale indexer state.

#### Results (from `doc/prefix-cache-test-results-summary.md`)

**Cold start:** first request scored **0.000** on prefix-cache across all pods (all weighted scores 1.500).

**Warm cache:** subsequent requests consistently scored **1.000** on the cached pod and **0.000** on others,
yielding **3.500 vs 1.500** weighted totals and consistent routing to the cached pod.

**Key takeaway:** the prefix-cache scorer correctly detects cache hits and dominates routing decisions when the
same long prompt repeats, demonstrating cache-aware scheduling behavior with the minimal test.

Example SchedulerInstall routing snippet:
```yaml
routing:
  enabled: true
  backendType: InferencePool
  inferencePool:
    name: gaie-inference-scheduling
    namespace: llm-d-sim
    port: 8200 # optional for InferencePool; required for Service backends
  httpRouteName: llm-d-inference-scheduling
  parentGateway:
    name: infra-inference-scheduling-inference-gateway
    namespace: llm-d-inference-scheduler
```

## Documentation

- `HANDS-ON.md`: end-to-end deployment and validation
- `doc/installation.md`: install and operator setup
- `doc/configuration.md`: configuration reference and sample manifests
- `doc/network-workflow.md`: network flow diagrams and path details
- `doc/trouble-shooting.md`: common issues and fixes
- `doc/development.md`: build, generate, and development tasks

## Compatibility

- Kubernetes v1.20+ (tested on modern clusters)
- Gateway API CRDs required for scheduler workflow
- Istio required for the default gateway class

## License

Apache License 2.0
