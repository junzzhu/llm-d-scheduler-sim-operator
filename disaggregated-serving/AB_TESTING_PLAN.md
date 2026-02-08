# A/B Testing Plan: NVLink vs Filesystem KV Cache Transfer

This document is the single place for:
- Gateway A/B sanity for KV cache transfer backend selection (`x-kv-backend`)
- Active-request-scorer-aware routing validation across proxy endpoints

## Current Status

- Backend A (NVLink / P2P NCCL): `vllm-p2p-proxy:8080` with `llm-d.ai/kv-backend=nvlink`
- Backend B (Filesystem KV Transfer): `vllm-fs-proxy:8080` with `llm-d.ai/kv-backend=fs`
- Scheduler route backend: `InferencePool/vllm-proxy-performance`
- EPP profile: `active-request-scorer` + `max-score-picker` (+ optional `by-label-selector`)

## Prerequisites

1. Backends are healthy:
```bash
kubectl get pods -n llm-d-vllm | rg 'vllm-(prefill|decode)-(fs|p2p)|vllm-(fs|p2p)-proxy'
```

2. Proxy labels are present:
```bash
kubectl get pods -n llm-d-vllm -l role=proxy --show-labels
```
Expected labels on proxy pods:
- FS proxy: `llm-d.ai/kv-backend=fs`
- P2P proxy: `llm-d.ai/kv-backend=nvlink`

3. Proxy metrics are exposed (required by `active-request-scorer`):
```bash
kubectl exec -n llm-d-vllm vllm-decode-fs -- sh -lc '
  curl -s -o /dev/null -w "fs=%{http_code}\n"  http://vllm-fs-proxy:8080/metrics
  curl -s -o /dev/null -w "p2p=%{http_code}\n" http://vllm-p2p-proxy:8080/metrics
'
```
Expected: both `200`.

## Step 1: Direct Backend Sanity (Bypass Gateway)

Filesystem:
```bash
kubectl exec -n llm-d-vllm vllm-decode-fs -- \
  curl -sS -X POST http://vllm-fs-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

P2P:
```bash
kubectl exec -n llm-d-vllm vllm-decode-p2p -- \
  curl -sS -X POST http://vllm-p2p-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

Expected:
- FS ID path shows `vllm-prefill-fs` and `vllm-decode-fs`
- P2P ID path shows `vllm-prefill-p2p:14579` and `vllm-decode-p2p:14579`

## Step 2: Gateway + InferencePool Wiring

Apply:
```bash
kubectl apply -f disaggregated-serving/gateway-routing/inferencepool-proxy-performance.yaml
kubectl apply -f disaggregated-serving/gateway-routing/schedulerinstall-proxy-performance.yaml
```

Verify:
```bash
kubectl get httproute -n llm-d-inference-scheduler llm-d-inference-scheduling -o yaml | rg -n "kind: InferencePool|name: vllm-proxy-performance"
kubectl get configmap -n llm-d-inference-scheduler gaie-inference-scheduling-epp-config -o jsonpath='{.data.epp-config\.yaml}' | rg -n "active-request-scorer|max-score-picker|by-label-selector|llm-d.ai/kv-backend"
```

## Step 3: Gateway Header-Based A/B Sanity

Port-forward gateway service:
```bash
kubectl port-forward -n llm-d-inference-scheduler \
  svc/infra-inference-scheduling-inference-gateway-istio 18080:80
```

NVLink header:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/completions \
  -H "Content-Type: application/json" \
  -H "x-kv-backend: nvlink" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

FS header:
```bash
curl -sS -X POST http://127.0.0.1:18080/v1/completions \
  -H "Content-Type: application/json" \
  -H "x-kv-backend: fs" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

Expected:
- `x-kv-backend: nvlink` reaches P2P backend
- `x-kv-backend: fs` reaches FS backend

## Step 4: Active-Request-Scorer Proxy Endpoint Test

Goal: prove scheduler path is selecting across proxy endpoints and not pinned.

Run controlled burst through gateway:
```bash
seq 1 20 | xargs -I{} -P5 sh -c '
curl -s -o /dev/null -w "%{http_code}\n" --max-time 25 \
  -H "Content-Type: application/json" \
  -X POST http://127.0.0.1:18080/v1/chat/completions \
  -d "{\"model\":\"Qwen/Qwen3-Coder-30B-A3B-Instruct\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}],\"max_tokens\":8,\"stream\":false}" \
  || true' | sort | uniq -c
```

Check proxy distribution deltas:
```bash
kubectl logs -n llm-d-vllm deploy/vllm-fs-proxy  --since=10m | rg -c 'POST /v1/chat/completions'
kubectl logs -n llm-d-vllm deploy/vllm-p2p-proxy --since=10m | rg -c 'POST /v1/chat/completions'
```

Check EPP has no selection failure:
```bash
kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp --since=10m \
  | rg -i 'InferencePoolResourceExhausted|failed to run scheduler profile'
```

Expected:
- Burst returns mostly/all `200`
- Both proxy counters increase during burst (not pinned to one proxy)
- No `InferencePoolResourceExhausted` errors

## Common Failure Signals

1. `InferencePoolResourceExhausted` in EPP logs:
- Usually proxy label mismatch or proxy metrics endpoint unavailable.

2. Proxy `/metrics` returns `404`:
- `active-request-scorer` cannot score endpoints; fix proxy script/metrics pass-through.

3. One proxy never gets traffic:
- Check labels on proxy pods and EPP `by-label-selector` criteria.

4. Header A/B mismatch:
- Verify `httproute-ab.yaml` and `ReferenceGrant` in `llm-d-vllm`.
