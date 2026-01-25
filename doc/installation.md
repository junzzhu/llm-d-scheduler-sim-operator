# Installation

## Prerequisites

- Kubernetes cluster v1.20+
- kubectl configured
- Go 1.22+ (for building from source)

## Install CRDs

```bash
kubectl apply -f config/crd/sim.llm-d.io_simulatordeployments.yaml
kubectl apply -f config/crd/sim.llm-d.io_schedulerinstalls.yaml
```

## Gateway API CRDs (Scheduler workflow)

```bash
kubectl apply -k ../llm-d-inference-scheduler/deploy/components/crds-gateway-api
```

## RBAC for EPP

```bash
./hack/create-epp-rbac.sh
```

## Run the Operator

```bash
./hack/redeploy-with-fixes.sh
```

For a step-by-step walkthrough, see `HANDS-ON.md`.
