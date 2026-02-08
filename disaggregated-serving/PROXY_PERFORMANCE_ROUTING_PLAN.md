# Performance-Based Routing Plan (EPP + Optional Hermes Labels)

## Objective

Use the scheduler gateway to pick backend endpoints by runtime load, without changing client request shape.

## Core Design

- Gateway route points to an `InferencePool` (not directly to a single Service).
- EPP evaluates candidate endpoints in that pool.
- `active-request-scorer` is the primary runtime signal (lower in-flight requests is preferred).
- `max-score-picker` selects the endpoint from computed scores.

This gives dynamic routing with no client-side label/header awareness.

## Why `active-request-scorer`

- Minimal and practical signal for live load.
- No extra client contract.
- Works across proxy endpoints (`role=proxy`) regardless of FS or P2P backend.

## Optional Backend Preference

If needed, apply a static filter before scoring:
- `by-label-selector` with `llm-d.ai/kv-backend=nvlink` to prefer NVLink-backed proxies.

Use this only when enforcing backend policy is intentional. For pure load balancing validation, keep scorer-only behavior.

## Hermes Role (Optional)

Hermes is not required for minimal scorer validation.

When used, Hermes-derived pod labels provide topology/policy hints that EPP can consume through label-based filters. Runtime selection still comes from EPP scoring.

## Config Profiles (Operator)

`SchedulerInstall.spec.epp.configProfile` controls behavior:
- `default`: existing behavior.
- `proxy-performance`: scorer-focused proxy routing.
- `proxy-performance-by-backend`: label filter + scorer routing.

## Operational Notes

- Proxy endpoints must be discoverable (`role=proxy`) and healthy.
- Proxy metrics endpoint must be available for scorer input.
- Generated EPP ConfigMaps are controller-managed; manual edits may be overwritten.

## Minimal Validation Plan (Only Prove `active-request-scorer` Works)

This is the smallest end-to-end scenario. Ignore A/B headers, Hermes labels, and backend preference logic.

### Scope

- Keep one gateway route: `llm-d-inference-scheduling`
- Route backend type: `InferencePool`
- Endpoint candidates: proxy pods (`role=proxy`)
- Picker signal: `active-request-scorer` only

### Steps

1. Sanity check both backends directly (before gateway test):
```bash
# FS backend sanity
kubectl exec -n llm-d-vllm vllm-decode-fs -- \
  curl -sS -X POST http://vllm-fs-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'

# P2P backend sanity
kubectl exec -n llm-d-vllm vllm-decode-p2p -- \
  curl -sS -X POST http://vllm-p2p-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

Expected:
- FS response `id` contains `vllm-prefill-fs` and `vllm-decode-fs`
- P2P response `id` contains `vllm-prefill-p2p` and `vllm-decode-p2p`

2. Apply minimal performance routing objects:
```bash
kubectl apply -f disaggregated-serving/gateway-routing/inferencepool-proxy-performance.yaml
kubectl apply -f disaggregated-serving/gateway-routing/schedulerinstall-proxy-performance.yaml
```

3. Verify gateway is using InferencePool (not direct Service/Pod):
```bash
kubectl get httproute -n llm-d-inference-scheduler llm-d-inference-scheduling -o yaml | rg -n "kind: InferencePool|name: vllm-proxy-performance"
```

4. Verify EPP config contains the scorer:
```bash
kubectl get configmap -n llm-d-inference-scheduler gaie-inference-scheduling-epp-config -o yaml | rg -n "active-request-scorer|decode-filter|configProfile"
```

5. Verify proxy endpoints are candidates:
```bash
kubectl get pods -n llm-d-vllm -l role=proxy -o wide
```

6. Run gateway load (burst) and observe picks:
```bash
# Example: send concurrent requests to gateway (replace endpoint as needed)
seq 1 50 | xargs -n1 -P20 -I{} curl -s -o /dev/null \
  -H 'Content-Type: application/json' \
  -X POST http://127.0.0.1:18080/v1/completions \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"hello","max_tokens":8}'
```

```bash
# In parallel, watch EPP logs for scoring/picking behavior
kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp --since=10m -f | rg -i 'score|endpoint|picked|active-request'
```

### Pass Criteria

- Requests go through `llm-d-inference-scheduling` -> `InferencePool vllm-proxy-performance`.
- EPP logs show endpoint scoring/picking activity from `active-request-scorer`.
- Under burst load, picks are not pinned to a single endpoint when multiple healthy proxy endpoints exist.
