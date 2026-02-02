# Network Workflow

This document explains the current scheduler + simulator network flow with the Istio Gateway API integration and the proxy Service bypass.

## Namespaces

- **Scheduler namespace**: `llm-d-inference-scheduler`
- **Simulator namespace**: `llm-d-sim`

## Key Resources

- **Gateway (Gateway API)**: `infra-inference-scheduling-inference-gateway`
- **Gateway Service (Istio data plane)**: `infra-inference-scheduling-inference-gateway-istio`
- **HTTPRoute**: routes requests from the Gateway to backend Service
- **ReferenceGrant**: allows cross-namespace backend reference
- **Proxy Service**: `gaie-inference-scheduling-proxy` (ClusterIP, sim namespace)
- **Decode pods**: simulator backends (role=decode)
- **EPP**: `gaie-inference-scheduling-epp` (scheduler namespace)

## 1) Scheduler Path (current default)

```
Client
  → Gateway Service (LoadBalancer/NodePort, :80)       [scheduler ns]
  → Istio Gateway Pod (data plane, :80)                [scheduler ns]
  → HTTPRoute                                          [scheduler ns]
  → Proxy Service gaie-inference-scheduling-proxy (:8200) [sim ns]
  → Simulator decode pods (:8200)                      [sim ns]
```

Notes:
- The HTTPRoute references the proxy Service in `llm-d-sim` and is allowed by ReferenceGrant.
- Traffic to simulator pods is plain HTTP on port 8200.

## 2) Direct Service Path (bypass scheduler)

```
Client
  → Proxy Service gaie-inference-scheduling-proxy (:8200) [sim ns]
  → Simulator decode pods (:8200)                      [sim ns]
```

Notes:
- This bypasses the scheduler Gateway and HTTPRoute.
- Useful for direct debugging or latency comparisons.

## 3) Scoring Path (when ext_proc is enabled)

```
Client
  → Gateway Service
  → Istio Gateway Pod (:80)
  → ext_proc call to EPP (gRPC :9002)
  → HTTPRoute
  → Proxy Service (:8200)
  → Simulator decode pods (:8200)
```

Notes:
- EPP listens on gRPC port 9002.
- Scoring is applied only when ext_proc is configured on the Gateway.

## Why the Proxy Service Bypass Exists

InferencePool headless service port discovery did not align with the simulator’s port (8200). The proxy Service provides a stable, explicit port and selector.

Impact:
- Routing is reliable.
- KV-scoring tests are **not meaningful** unless ext_proc is enabled and routing goes through InferencePool/EPP selection.

## Ports

- **Gateway Service**: 80 (HTTP), 15021 (status)
- **Proxy Service**: 8200 (HTTP)
- **EPP**: 9002 gRPC, 9003 health
- **Simulator pods**: 8200 HTTP, `/metrics`
