# Instrumented EPP Scoring Analysis Plan (`llm-d-inference-scheduler:local`)

## Goal

Use an instrumented EPP image to make scoring decisions observable at request time, especially for `active-request-scorer`.

This plan is for debugging and validation, not production rollout.

## Scope

- Runtime target: `gaie-inference-scheduling-epp` in `llm-d-inference-scheduler`
- Image target: `llm-d-inference-scheduler:local`
- Focused signal: per-endpoint scorer values and final picker decision

## Instrumentation Points

Add logs in `llm-d-inference-scheduler/_deps/gateway-api-inference-extension/`:

- `pkg/epp/scheduling/framework/scheduler_profile.go`
  - Log candidate endpoints before filtering.
  - Log scorer outputs per endpoint (including `active-request-scorer` values).
  - Log chosen endpoint after picker runs.
- `pkg/epp/handlers/server.go`
  - Log request correlation keys (path/model/request id if available).
  - Log selected endpoint returned to ext_proc stream.

Keep logs structured (`zap` fields) so request-by-request correlation is easy.

## Build Local Instrumented Image

From repo root:

```bash
docker build -t llm-d-inference-scheduler:local \
  -f llm-d-inference-scheduler/Dockerfile.epp \
  llm-d-inference-scheduler
```

Load image into your cluster runtime:

```bash
# kind
kind load docker-image llm-d-inference-scheduler:local

# minikube
minikube image load llm-d-inference-scheduler:local
```

## Deploy the Instrumented Image

Preferred: patch `SchedulerInstall` so operator-owned state remains consistent.

```bash
kubectl patch schedulerinstall -n llm-d-inference-scheduler llm-sched-install \
  --type merge \
  -p '{"spec":{"epp":{"image":"llm-d-inference-scheduler:local"}}}'
```

Then verify:

```bash
kubectl get deploy -n llm-d-inference-scheduler gaie-inference-scheduling-epp \
  -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
```

Expected: `llm-d-inference-scheduler:local`

## Run Scoring Analysis

1. Confirm scorer config and candidate pool:
```bash
kubectl get configmap -n llm-d-inference-scheduler gaie-inference-scheduling-epp-config \
  -o jsonpath='{.data.epp-config\.yaml}' | rg -n "active-request-scorer|by-label-selector|max-score-picker"

kubectl get pods -n llm-d-vllm -l role=proxy --show-labels
```

2. Generate concurrent gateway load:
```bash
seq 1 20 | xargs -I{} -P5 sh -c '
curl -s -o /dev/null --max-time 25 \
  -H "Content-Type: application/json" \
  -X POST http://127.0.0.1:18080/v1/chat/completions \
  -d "{\"model\":\"Qwen/Qwen3-Coder-30B-A3B-Instruct\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}],\"max_tokens\":8,\"stream\":false}"'
```

3. Capture instrumented EPP logs:
```bash
kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp --since=10m -f | \
  rg -i "active-request|score|candidate|picked|endpoint|profile"
```

## What to Verify in Logs

- Candidate endpoint set before scoring.
- `active-request-scorer` values per endpoint for the same request.
- Final picker output and selected endpoint.
- No selection failures:
  - `InferencePoolResourceExhausted`
  - `failed to run scheduler profile`

## Rollback

Restore the standard image:

```bash
kubectl patch schedulerinstall -n llm-d-inference-scheduler llm-sched-install \
  --type merge \
  -p '{"spec":{"epp":{"image":"ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0"}}}'
```

Confirm rollout:

```bash
kubectl rollout status deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler
```

## Notes

- If the operator reconciles frequently, patch `SchedulerInstall` (not Deployment directly).
- Keep instrumentation behind concise debug statements to avoid log flood under load.
