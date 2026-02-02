# LLM-D Scheduler-Sim Operator

A minimal Kubernetes operator that simplifies deployment and management of llm-d inference scheduler and simulator deployment.

## Overview

This operator automates both llm-d **simulator** and **scheduler** setup as documented in [llm-d-scheduler-sim](https://github.com/llm-d/llm-d-scheduler-sim) and [llm-d-inference-scheduler](https://github.com/llm-d/llm-d-inference-scheduler) projects. It replaces manual simulator scaling, service wiring, Gateway API routing, and Istio load-balancing with declarative Custom Resources (`SimulatorDeployment` and `SchedulerInstall`). 

The integrated scheduler+simulator flow makes it easier to iterate on scheduler scoring and routing logic, and keeps experiments reproducible across environments.

## Features

- **LLM-D Stack Deployment**: Deploy llm-d inference scheduler and simulator architecture
- **EPP (Endpoint Picker)**: Automated deployment of endpoint picker service
- **Inference Gateways**: Support for both standard (kgateway) and Istio gateways
- **Separate Prefill/Decode Stages**: Independent scaling and configuration for prefill and decode
- **Automated Deployment**: Creates simulator deployments with proper labels
- **Service Management**: Automatically creates services for all components
- **Scaling**: Simple replica management through CR spec
- **Load Balancing**: Optional Istio DestinationRule configuration

## Current Status

- Scheduler and simulator workflows are both supported via CRDs (`SchedulerInstall` and `SimulatorDeployment`).
- Default scheduler gateway class is **Istio**; routing uses Gateway API with an HTTPRoute to the proxy Service.
- Current integration uses a **proxy Service bypass** for InferencePool headless service limitations.
- Scoring requires ext_proc wiring (see scheduler workflow docs).

## Workflow Overview

**Scheduler path (current default)**  
Client → Scheduler Gateway (Istio data plane) → HTTPRoute → Proxy Service (`gaie-inference-scheduling-proxy`) → Simulator decode pods

**Direct service path (bypass scheduler)**  
Client → Proxy Service (`gaie-inference-scheduling-proxy`) → Simulator decode pods

**Scoring path (when ext_proc is enabled)**  
Client → Scheduler Gateway → ext_proc (EPP) → HTTPRoute → Proxy Service → Simulator decode pods

## KV Cache-Aware Scoring (ext_proc + EPP)

Enable the EPP service and EnvoyFilter to activate scoring, then validate the ext_proc listener and EPP logs.
See `doc/future-work.md` for the EnvoyFilter manifest, apply steps, and validation commands.

Note: Scoring depends on the scheduler path using InferencePool state. If routing bypasses the scheduler
or InferencePool is not populated, EPP may return default/neutral scores and routing will still follow the
proxy Service selector.

## Action Plan: Enable InferencePool + KV-Scoring

1) Fix InferencePool port alignment
   - Ensure the InferencePool backend port matches the simulator decode Service port.
   - In this repo, decode Services expose port `8200` with name `http`. Keep that consistent in InferencePool
     backendRefs (use `port: 8200` when a port is required).
   - Note: `port` is required for Service backendRefs but optional for InferencePool backendRefs. Include
     `port: 8200` for Service; use it for InferencePool only if your controller/API expects it.
   - If the InferencePool controller expects a headless Service, verify it resolves to pod endpoints on `8200`.

2) Route scheduler HTTPRoute to InferencePool
   - Ensure the InferencePool CRD/controller is installed and the pool exists before switching backendRefs.
   - Update HTTPRoute backendRefs to point at the InferencePool (not the proxy Service).
   - Keep the ReferenceGrant in the simulator namespace allowing HTTPRoute → InferencePool.

3) Enable EPP and point it at the InferencePool
   - Set `spec.epp.enabled: true`.
   - Set `spec.epp.poolName` and `spec.epp.poolNamespace` to the InferencePool you route to.

4) Enable ext_proc at the Gateway
   - Turn on `spec.envoyFilter.enabled: true` (or apply the EnvoyFilter manifest in `doc/future-work.md`).
   - Set `spec.envoyFilter.workloadSelector` (or rely on the default selector derived from `spec.gateway.name`).
   - Validate ext_proc listener and EPP logs as described in `doc/future-work.md`.

5) Validate scoring-based scheduling
   - Send test traffic through the scheduler gateway.
   - Confirm EPP logs show scoring decisions and that backend selection changes with cache state.

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
- `doc/scheduler-workflow.md`: scheduler vs direct service paths, round-robin, scoring, bypass behavior
- `doc/trouble-shooting.md`: common issues and fixes
- `doc/development.md`: build, generate, and development tasks
- `doc/future-work.md`: optional extensions (e.g., EnvoyFilter for ext_proc)

## Compatibility

- Kubernetes v1.20+ (tested on modern clusters)
- Gateway API CRDs required for scheduler workflow
- Istio required for the default gateway class

## Troubleshooting

See `doc/trouble-shooting.md`.

## Comparison with Manual Approach

| Aspect | Manual | With Operator |
|--------|--------|---------------|
| Deployment | Multiple kubectl commands | Single CR apply |
| Scaling | `kubectl scale` | Update `replicas` in CR |
| Load Balancing | Manual DestinationRule | Set `loadBalancing.enabled: true` |
| Status Checking | Multiple kubectl get commands | `kubectl get simulatordeployment` |
| Configuration | Multiple YAML files | Single CR |
| GitOps | Difficult to track | Declarative, version-controlled |


## License

Apache License 2.0
