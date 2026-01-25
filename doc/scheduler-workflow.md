# Scheduler and Simulator Workflow

## Current Default Path

With the current operator defaults, the scheduler path is:
`Gateway (Istio class) -> HTTPRoute -> proxy Service -> simulator pods`

The Gateway data plane is Istio, but the routing path is the scheduler path because requests go through the scheduler gateway and HTTPRoute.

## Direct Service Path

The direct service path bypasses the scheduler gateway and calls the proxy Service directly:
`Client -> gaie-inference-scheduling-proxy -> simulator pods`

## Round-Robin Behavior

Routing behavior depends on which layer is making the decision:
- Scheduler path: EPP chooses the backend based on scorers, so it is not round-robin by default.
- Direct service path: Kubernetes Service load-balancing is used (roughly round-robin per connection).
- Istio balancing: default is `LEAST_REQUEST` unless a DestinationRule sets `ROUND_ROBIN`.

Check the active policy:
```bash
kubectl get destinationrule gaie-inference-scheduling-proxy-lb -n llm-d-sim \
  -o jsonpath='{.spec.trafficPolicy.loadBalancer}'
```

## Scoring and ext_proc

Scoring works when the Gateway is configured to call EPP via ext_proc (Gateway API Inference Extension or equivalent filter configuration). The operator deploys EPP and the scheduler Gateway/HTTPRoute, but does not inject ext_proc configuration.

See `doc/future-work.md` for the EnvoyFilter option on Istio.

## Bypass Setup and KV-Scoring Tests

The current integration uses a proxy Service bypass to avoid InferencePool headless service/port issues:
`HTTPRoute -> Service (gaie-inference-scheduling-proxy) -> simulator pods`

This makes routing reliable, but it bypasses InferencePool-based selection, so KV-scoring tests are not meaningful.

To test KV-scoring:
1) Enable ext_proc on the Gateway, and
2) Route through an InferencePool backendRef instead of the proxy Service.
