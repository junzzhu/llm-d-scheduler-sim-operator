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

## Documentation

- `HANDS-ON.md`: end-to-end deployment and validation
- `doc/installation.md`: install and operator setup
- `doc/configuration.md`: configuration reference and sample manifests
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
