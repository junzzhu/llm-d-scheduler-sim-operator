# Development

## Build

```bash
go build -o bin/manager main.go
```

## Generate CRDs

```bash
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
controller-gen crd paths="./api/..." output:crd:dir=./config/crd
```

## EPP Debug Image Rollout

Use this when you need to rebuild the EPP image and force the SchedulerInstall to pick it up.

1. Build the debug image:

```bash
DOCKER_BUILDKIT=0 EPP_TAG=local make image-build-epp
```

2. Load into minikube:

```bash
minikube image load ghcr.io/llm-d/llm-d-inference-scheduler:local
```

3. Patch SchedulerInstall to use it (and revert when done):

```bash
kubectl patch schedulerinstall llm-sched-install -n llm-d-inference-scheduler \
  --type merge \
  -p '{"spec":{"epp":{"image":"ghcr.io/llm-d/llm-d-inference-scheduler:local"}}}'

kubectl patch schedulerinstall llm-sched-install -n llm-d-inference-scheduler \
  --type merge \
  -p '{"spec":{"epp":{"image":"ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0"}}}'

kubectl annotate schedulerinstall llm-sched-install -n llm-d-inference-scheduler \
  reconcile-at="$(date +%s)" --overwrite
```

4. Watch rollout:

```bash
kubectl rollout status deploy/gaie-inference-scheduling-epp -n llm-d-inference-scheduler
```

5. Test and tail logs:

```bash
kubectl run -n llm-d-inference-scheduler curl-gw --rm -i --restart=Never --image=curlimages/curl -- \
  -m 15 --connect-timeout 3 -v \
  http://10.97.89.229/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"random","prompt":"test","max_tokens":5}'

kubectl logs -n llm-d-inference-scheduler deploy/gaie-inference-scheduling-epp -f | rg -n "request body|send request body responses|send response"
```
