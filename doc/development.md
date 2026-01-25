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
