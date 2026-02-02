#!/bin/bash
set -e

NS_SIM=${NS_SIM:-llm-d-sim}

echo "=== Creating RBAC for EPP ==="

# Create ServiceAccount, Role, and RoleBinding for EPP
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gaie-sim-epp
  namespace: ${NS_SIM}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: gaie-sim-epp
  namespace: ${NS_SIM}
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["inference.networking.k8s.io","inference.networking.x-k8s.io"]
  resources: ["inferencepools", "inferencemodels", "inferenceobjectives"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["inference.networking.k8s.io","inference.networking.x-k8s.io"]
  resources: ["inferencepools/status", "inferencemodels/status", "inferenceobjectives/status"]
  verbs: ["get", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: gaie-sim-epp
  namespace: ${NS_SIM}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gaie-sim-epp
subjects:
- kind: ServiceAccount
  name: gaie-sim-epp
  namespace: ${NS_SIM}
EOF

echo ""
echo "Updating EPP deployment to use the ServiceAccount..."
kubectl patch deployment gaie-sim-epp -n ${NS_SIM} -p '{"spec":{"template":{"spec":{"serviceAccountName":"gaie-sim-epp"}}}}' || echo "Deployment not found, skipping patch (will be set by operator)"

echo ""
echo "Waiting for EPP pod to restart..."
if kubectl get deployment gaie-sim-epp -n ${NS_SIM} &> /dev/null; then
  kubectl rollout status deployment/gaie-sim-epp -n ${NS_SIM} --timeout=60s
fi

echo ""
echo "✅ RBAC configured successfully"
echo ""
echo "Check EPP logs:"
echo "  kubectl logs -f deployment/gaie-sim-epp -n ${NS_SIM}"
