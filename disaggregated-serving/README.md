# Disaggregated Serving

Kubernetes manifests for vLLM disaggregated prefill/decode serving with two KV cache transfer backends and gateway-based A/B testing.

## Directory Structure

```
disaggregated-serving/
├── README.md                      # This file
├── AB_TESTING_PLAN.md             # A/B testing plan and implementation
├── vllm/                          # vLLM disaggregated serving manifests
│   ├── fs-kv-same-node.yaml       # Filesystem backend pods
│   ├── fs-proxy-deployment.yaml   # Filesystem proxy
│   ├── fs-proxy.py                # Filesystem proxy script
│   ├── p2p-nccl-same-node.yaml    # P2P NCCL backend pods
│   ├── p2p-proxy-ubi.yaml         # P2P NCCL proxy
│   ├── p2p-proxy.py               # P2P NCCL proxy script
│   └── p2p-proxy.Dockerfile       # P2P NCCL proxy image
└── gateway-routing/               # Gateway routing for A/B testing
    └── httproute-ab.yaml          # HTTPRoute for header-based routing
```

## KV Cache Transfer Backends

### 1. Filesystem Backend
- **Manifests**: [`vllm/fs-kv-same-node.yaml`](vllm/fs-kv-same-node.yaml), [`vllm/fs-proxy-deployment.yaml`](vllm/fs-proxy-deployment.yaml)
- Uses `ExampleConnector` with shared `emptyDir` storage at `/kv_cache`
- Works on any hardware without special requirements
- Higher latency (disk I/O)

### 2. P2P NCCL Backend
- **Manifests**: [`vllm/p2p-nccl-same-node.yaml`](vllm/p2p-nccl-same-node.yaml), [`vllm/p2p-proxy-ubi.yaml`](vllm/p2p-proxy-ubi.yaml)
- Uses `P2pNcclConnector` with NCCL P2P transfer via NVLink
- Direct GPU-to-GPU KV cache transfer
- Requires NVLink-capable GPUs (e.g., H100 NVL) on same node
- Ultra-low latency (~0.092ms)

## Deployment

### Deploy Backends

```bash
# Filesystem Backend
kubectl apply -f disaggregated-serving/vllm/fs-kv-same-node.yaml
kubectl apply -f disaggregated-serving/vllm/fs-proxy-deployment.yaml

# P2P NCCL Backend
kubectl apply -f disaggregated-serving/vllm/p2p-nccl-same-node.yaml
kubectl apply -f disaggregated-serving/vllm/p2p-proxy-ubi.yaml
```

### Deploy Gateway Routing

```bash
kubectl apply -f disaggregated-serving/gateway-routing/httproute-ab.yaml
```

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

### Gateway A/B Testing

Port-forward the gateway:
```bash
kubectl port-forward -n llm-d-inference-scheduler \
  svc/infra-inference-scheduling-inference-gateway-istio 18080:80
```

Test with header-based routing:
```bash
# Test NVLink backend
curl -X POST http://localhost:18080/v1/completions \
  -H "Content-Type: application/json" \
  -H "x-kv-backend: nvlink" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'

# Test Filesystem backend
curl -X POST http://localhost:18080/v1/completions \
  -H "Content-Type: application/json" \
  -H "x-kv-backend: fs" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

## Request ID Format

Both backends use the same request ID format for tracking:
```
cmpl-___prefill_addr_{prefill_service}___decode_addr_{decode_service}_{uuid}
```

**Examples:**
- Filesystem: `cmpl-___prefill_addr_vllm-prefill-fs___decode_addr_vllm-decode-fs_{uuid}`
- P2P NCCL: `cmpl-___prefill_addr_vllm-prefill-p2p:14579___decode_addr_vllm-decode-p2p:14579_{uuid}`

## Gateway Routing

Header-based routing through the scheduler gateway:
- `x-kv-backend: nvlink` → Routes to P2P NCCL backend
- `x-kv-backend: fs` → Routes to Filesystem backend
- No header → Falls through to default scheduler route

## Status

- ✅ Filesystem Backend: Operational
- ✅ P2P NCCL Backend: Operational  
- ✅ Gateway Routing: Deployed and verified
- 🔄 Performance Comparison: In progress

## Documentation

- **[AB_TESTING_PLAN.md](AB_TESTING_PLAN.md)** - Complete A/B testing setup, implementation, and performance comparison plan