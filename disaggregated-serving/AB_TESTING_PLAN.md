# A/B Testing Plan: NVLink vs Filesystem KV Cache Transfer

## Current Status

✅ **Both backends are operational and tested:**

### Backend A: P2P NCCL (NVLink)
- **Proxy Service**: `vllm-p2p-proxy:8080` (namespace: `llm-d-vllm`)
- **Prefill Pod**: `vllm-prefill-p2p`
- **Decode Pod**: `vllm-decode-p2p`
- **KV Transfer**: NCCL P2P via NVLink (~0.092ms)
- **Request ID Format**: `cmpl-___prefill_addr_vllm-prefill-p2p:14579___decode_addr_vllm-decode-p2p:14579_{uuid}`

### Backend B: Filesystem
- **Proxy Service**: `vllm-fs-proxy:8080` (namespace: `llm-d-vllm`)
- **Prefill Pod**: `vllm-prefill-fs`
- **Decode Pod**: `vllm-decode-fs`
- **KV Transfer**: Shared `emptyDir` at `/kv_cache`
- **Request ID Format**: `cmpl-___prefill_addr_vllm-prefill-fs___decode_addr_vllm-decode-fs_{uuid}`

## Next Steps for A/B Testing

### Step 1: Reuse the Existing Scheduler Gateway
Reuse the scheduler gateway created by `SchedulerInstall` (Istio Gateway API).

Verify it exists:
```bash
kubectl get gateway -n llm-d-inference-scheduler infra-inference-scheduling-inference-gateway
kubectl get svc -n llm-d-inference-scheduler infra-inference-scheduling-inference-gateway-istio
```

Port-forward it for testing (recommended):
```bash
kubectl port-forward -n llm-d-inference-scheduler \
  svc/infra-inference-scheduling-inference-gateway-istio 18080:80
```

Note: This A/B plan uses **header-based routing directly to Services**. Requests will go through the
same gateway, but **EPP scoring/picking is not what selects the A vs B backend** (the HTTPRoute match does).

### Step 2: Verify ReferenceGrant (Cross-Namespace BackendRefs)
The existing ReferenceGrant `allow-scheduler-httproute` already permits cross-namespace Service access:

```bash
kubectl get referencegrant allow-scheduler-httproute -n llm-d-vllm -o yaml
```

This grant allows HTTPRoutes from `llm-d-inference-scheduler` to reference Services in `llm-d-vllm`. ✅

### Step 3: Apply HTTPRoute (Header-Based A/B Routing)
Apply the HTTPRoute manifest:

```bash
kubectl apply -f disaggregated-serving/gateway-routing/httproute-ab.yaml
```

Verify it's accepted:

```bash
kubectl get httproute vllm-ab-test -n llm-d-inference-scheduler
```

Expected status:
- `Accepted: True` - Route was valid
- `ResolvedRefs: True` - All references resolved

The HTTPRoute routes based on the `x-kv-backend` header:
- `x-kv-backend: nvlink` → `vllm-p2p-proxy:8080`
- `x-kv-backend: fs` → `vllm-fs-proxy:8080`
- No header → Falls through to default scheduler route

### Step 4: Testing Commands

#### Test NVLink Backend (explicit header)
```bash
curl -X POST http://localhost:18080/v1/completions \
  -H "Content-Type: application/json" \
  -H "x-kv-backend: nvlink" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

**Expected Response ID**: Contains `vllm-prefill-p2p:14579` and `vllm-decode-p2p:14579`

#### Test Filesystem Backend (explicit header)
```bash
curl -X POST http://localhost:18080/v1/completions \
  -H "Content-Type: application/json" \
  -H "x-kv-backend: fs" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

**Expected Response ID**: Contains `vllm-prefill-fs` and `vllm-decode-fs`
