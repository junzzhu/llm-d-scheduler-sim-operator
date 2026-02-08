# Disaggregated Serving

Kubernetes manifests for vLLM disaggregated prefill/decode serving with two KV cache transfer backends and gateway-based A/B testing.


## KV Cache Transfer Backends

### 1. Filesystem Backend
- **Manifests**: [`vllm/fs-kv-same-node.yaml`](vllm/fs-kv-same-node.yaml)
- Uses `ExampleConnector` with shared `emptyDir` storage at `/kv_cache`
- Works on any hardware without special requirements
- Higher latency (disk I/O)

### 2. P2P NCCL Backend
- **Manifests**: [`vllm/p2p-nccl-same-node.yaml`](vllm/p2p-nccl-same-node.yaml), [`vllm/p2p-proxy-ubi.yaml`](vllm/p2p-proxy-ubi.yaml)
- Uses `P2pNcclConnector` with NCCL P2P transfer via NVLink
- Direct GPU-to-GPU KV cache transfer
- Requires NVLink-capable GPUs (e.g., H100 NVL) on same node
- Ultra-low latency (~0.092ms)


## Testing

### Direct Backend Testing

**Test Filesystem Backend:**
```bash
kubectl exec -n llm-d-vllm vllm-decode-fs -- \
  curl -sS -X POST http://vllm-fs-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

**Test P2P NCCL Backend:**
```bash
kubectl exec -n llm-d-vllm vllm-decode-p2p -- \
  curl -sS -X POST http://vllm-p2p-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

For end-to-end test flows:
- NVLink vs Filesystem A/B flow: [`AB_TESTING_PLAN.md`](AB_TESTING_PLAN.md)
- Proxy performance routing flow: [`PROXY_PERFORMANCE_ROUTING_PLAN.md`](PROXY_PERFORMANCE_ROUTING_PLAN.md)

## Active-Request-Scorer Test Design

This design validates that scheduler routing reacts to live load across proxy endpoints.

- Keep the client contract unchanged (standard OpenAI-style API calls to gateway).
- Route through `HTTPRoute -> InferencePool` so endpoint selection is handled by EPP, not static service routing.
- Use proxy endpoints (`role=proxy`) as the scheduling candidates:
  - `vllm-fs-proxy` (Filesystem KV transfer)
  - `vllm-p2p-proxy` (NVLink/P2P KV transfer)
- Use `active-request-scorer` as the runtime signal (in-flight requests) to avoid pinning traffic to one endpoint.

### Success Criteria

- Scheduler route backend is `InferencePool` (not direct Service-only routing).
- EPP profile includes `active-request-scorer`.
- Under concurrent traffic, requests are observed on both proxy endpoints when both are healthy.
- No scheduler selection failure signals (for example `InferencePoolResourceExhausted`).

## Request ID Format

Both backends use the same request ID format for tracking:
```
cmpl-___prefill_addr_{prefill_service}___decode_addr_{decode_service}_{uuid}
```

**Examples:**
- Filesystem: `cmpl-___prefill_addr_vllm-prefill-fs___decode_addr_vllm-decode-fs_{uuid}`
- P2P NCCL: `cmpl-___prefill_addr_vllm-prefill-p2p:14579___decode_addr_vllm-decode-p2p:14579_{uuid}`

## Status

- ✅ Filesystem Backend: Operational
- ✅ P2P NCCL Backend: Operational  
- ✅ Gateway Routing: Deployed and verified
- 🔄 Performance Comparison: In progress

## Documentation

- **[AB_TESTING_PLAN.md](AB_TESTING_PLAN.md)** - Complete A/B testing setup and implementation
- **[PROXY_PERFORMANCE_ROUTING_PLAN.md](PROXY_PERFORMANCE_ROUTING_PLAN.md)** - Performance-based routing plan with opt-in proxy/EPP mode
