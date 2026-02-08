# Performance-Aware Routing in Disaggregated Serving

This directory is the working area for disaggregated vLLM serving experiments, including manifests, routing configs, and test plans.

Current focus in this repo area:
- Two KV transfer backends for prefill/decode: `P2P NCCL (NVLink)` and `Filesystem`
- Gateway + `InferencePool` routing through EPP with `active-request-scorer`
- A/B validation of proxy endpoint selection (`vllm-p2p-proxy` vs `vllm-fs-proxy`)
- A suspected vLLM V1 KV-transfer BUG that is blocking larger-scale tests

## GPU Topology

The test OpenShift GPU cluster has the following architecture, based on [Hermes](https://github.com/llm-d-incubation/hermes) scan results:

- `worker-A`: 2x `NVIDIA-H100-NVL` (NVLink)
- `worker-B`: 2x `NVIDIA-H100`
- RDMA: not detected on either node


```text
+----------------------------------------------------------------------------------+
|                          OpenShift GPU Cluster                                   |
+----------------------------------------------------------------------------------+
| worker-A                                                                         |
|  +-----------------------------------------------------------------------------+ |
|  |                                                                             | |
|  |  +---------------------+        NVLink        +---------------------+       | |
|  |  | GPU0: H100-NVL      | <------------------> | GPU1: H100-NVL      |       | |
|  |  +---------------------+       (~600 GB/s)    +---------------------+       | |
|  +-----------------------------------------------------------------------------+ |
|                           /\                              /\                     |
|                           ||         Inter-node           ||                     |
|                           ||          Ethernet            ||                     |
|                           ||        (~10-25 GB/s)         ||                     |
| worker-B                  \/                              \/                     |
|  +-----------------------------------------------------------------------------+ |
|  |                                                                             | |
|  |  +---------------------+  CPU / PCIe / FS     +---------------------+       | |
|  |  | GPU0: H100          | <------------------> | GPU1: H100          |       | |
|  |  +---------------------+     (~1-10 GB/s)     +---------------------+       | |
|  +-----------------------------------------------------------------------------+ |
+----------------------------------------------------------------------------------+
```


## KV Cache Transfer Backends

Placement implication for P2P KV transfer:
- The NVLink-optimized P2P path should co-locate prefill/decode on `worker-A` to benefit from intra-node NVLink between the two H100-NVL GPUs.

### 1. P2P NCCL Backend
- **Manifests**: [`vllm/p2p-nccl-same-node.yaml`](vllm/p2p-nccl-same-node.yaml)
- Uses `P2pNcclConnector` with NCCL P2P transfer via NVLink
- Direct GPU-to-GPU KV cache transfer
- Requires NVLink-capable GPUs (e.g., H100 NVL) on same node
- Ultra-low latency (~0.092ms)


### 2. Filesystem Backend
- **Manifests**: [`vllm/fs-kv-same-node.yaml`](vllm/fs-kv-same-node.yaml)
- Uses `ExampleConnector` with shared `emptyDir` storage at `/kv_cache`
- Works on any hardware without special requirements
- Higher latency (disk I/O)

## Testing

### Direct Backend Testing


**Test P2P NCCL Backend:**
```bash
kubectl exec -n llm-d-vllm vllm-decode-p2p -- \
  curl -sS -X POST http://vllm-p2p-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

**Test Filesystem Backend:**
```bash
kubectl exec -n llm-d-vllm vllm-decode-fs -- \
  curl -sS -X POST http://vllm-fs-proxy:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"Qwen/Qwen3-Coder-30B-A3B-Instruct","prompt":"Hello","max_tokens":5}'
```

For end-to-end test flows:
- NVLink vs Filesystem A/B flow: [`AB_TESTING_PLAN.md`](AB_TESTING_PLAN.md)
- Proxy performance routing flow: [`PROXY_PERFORMANCE_ROUTING_PLAN.md`](PROXY_PERFORMANCE_ROUTING_PLAN.md)

## Active-Request-Scorer Test Design

This design validates that scheduler routing reacts to live load across proxy endpoints.

- Keep the client using standard OpenAI-style API calls to gateway.
- Route through `HTTPRoute -> InferencePool` so endpoint selection is handled by EPP, not static service routing.
- Use proxy endpoints (`role=proxy`) as the scheduling candidates:
  - `vllm-fs-proxy` (Filesystem KV transfer)
  - `vllm-p2p-proxy` (NVLink/P2P KV transfer)
- Use `active-request-scorer` as the runtime signal (in-flight requests) to avoid pinning traffic to one endpoint.

### Current Validation Settings (`proxy-performance`)

Goal: verify that `active-request-scorer` can choose between both proxy backends (`fs` and `p2p`) under the same gateway route.

Runtime settings used:
- `SchedulerInstall.spec.epp.configProfile=proxy-performance`
- Runtime EPP config contains only:
  - `active-request-scorer`
  - `max-score-picker`
  - no `by-label-selector`

Why this matters:
- When `by-label-selector` is present with `llm-d.ai/kv-backend=nvlink`, FS is filtered out before scoring.
- Removing that filter allows both proxies to remain in the candidate set.

```bash
# 1) Ensure SchedulerInstall uses proxy-performance
kubectl patch schedulerinstall -n llm-d-inference-scheduler llm-sched-install --type=merge \
  -p '{"spec":{"epp":{"configProfile":"proxy-performance"}}}'

# 2) Verify runtime EPP config (should have scorer + picker, no by-label-selector)
kubectl get configmap -n llm-d-inference-scheduler gaie-inference-scheduling-epp-config \
  -o jsonpath='{.data.epp-config\.yaml}' | rg -n 'active-request-scorer|by-label-selector|max-score-picker'

# 3) Restart EPP so runtime config is reloaded
kubectl rollout restart -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp
kubectl rollout status -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp --timeout=180s

# 4) Generate concurrent gateway load
seq 1 60 | xargs -I{} -P12 sh -c '
curl -s -o /dev/null --max-time 35 \
  -H "Content-Type: application/json" \
  -X POST http://127.0.0.1:18080/v1/chat/completions \
  -d "{\"model\":\"Qwen/Qwen3-Coder-30B-A3B-Instruct\",\"messages\":[{\"role\":\"user\",\"content\":\"hello-{}\"}],\"max_tokens\":8,\"stream\":false}"'

# 5) Verify EPP scoring decisions include BOTH proxies
kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp --since=10m | \
  rg 'Scoring decision' | rg -o 'selected\":\[[^]]+\]' | \
  sed 's/selected\":\[\"//;s/\"\]//' | sort | uniq -c | sort -nr
```

Observed run (validated):
- Request outcome: `60/60` returned `200`.
- EPP loaded profile with `Filters: []` and `Scorers: [active-request-scorer]`.
- Selection split from `Scoring decision` logs:
  - `32` -> `vllm-p2p-proxy-*`
  - `28` -> `vllm-fs-proxy-*`
- Interpretation: both backends were present in `scores`, and FS participated in final picks (not filtered out).

Current blocker for larger-scale tests:
- A suspected vLLM V1 KV-transfer bug (decode-side `IndexError` in `gpu_model_runner.py`) causes intermittent backend instability under heavier/longer campaigns.
- Until this is resolved, treat large-scale performance comparisons as provisional.
- Details and triage notes: [`../doc/vllm-v1-kv-transfer-index-error.md`](../doc/vllm-v1-kv-transfer-index-error.md).

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


## Documentation

- **[AB_TESTING_PLAN.md](AB_TESTING_PLAN.md)** - Complete A/B testing setup and implementation
- **[PROXY_PERFORMANCE_ROUTING_PLAN.md](PROXY_PERFORMANCE_ROUTING_PLAN.md)** - Performance-based routing plan with opt-in proxy/EPP mode
- **[INSTRUMENTED_EPP_SCORING_PLAN.md](INSTRUMENTED_EPP_SCORING_PLAN.md)** - Build/deploy and use an instrumented EPP image to inspect scoring decisions in detail
- **[../doc/vllm-v1-kv-transfer-index-error.md](../doc/vllm-v1-kv-transfer-index-error.md)** - Suspected vLLM V1 KV-transfer engine bug and investigation notes
