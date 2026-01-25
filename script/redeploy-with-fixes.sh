#!/bin/bash
set -e

NS_SIM=${NS_SIM:-llm-d-sim}

echo "=== Redeploying llm-d-scheduler-sim-operator with EPP/Gateway fixes ==="

# Step 1: Delete existing SimulatorDeployment
echo ""
echo "Step 1: Deleting existing SimulatorDeployment..."
kubectl delete simulatordeployment --all -n ${NS_SIM} --ignore-not-found=true

# Wait for resources to be cleaned up
echo "Waiting for resources to be deleted..."
sleep 5

# Step 2: Stop the running operator (if any)
echo ""
echo "Step 2: Stopping any running operator processes..."
pkill -f "bin/manager" || true
sleep 2

# Step 3: Rebuild the operator
echo ""
echo "Step 3: Rebuilding operator..."
go build -o bin/manager main.go

# Step 4: Start the operator in background
echo ""
echo "Step 4: Starting operator..."
./bin/manager > /tmp/operator.log 2>&1 &
OPERATOR_PID=$!
echo "Operator started with PID: $OPERATOR_PID"
echo "Logs: tail -f /tmp/operator.log"

# Wait for operator to be ready
echo "Waiting for operator to initialize..."
sleep 5

# Step 5: Apply the SimulatorDeployment CR
echo ""
echo "Step 5: Applying SimulatorDeployment CR..."
kubectl apply -f config/samples/sim_v1alpha1_simulatordeployment_full.yaml

# Step 6: Watch the status
echo ""
echo "Step 6: Watching deployment status..."
echo "Run these commands to monitor:"
echo "  kubectl get pods -n ${NS_SIM} -w"
echo "  kubectl get configmap -n ${NS_SIM}"
echo "  kubectl describe deployment gaie-sim-epp -n ${NS_SIM}"
echo "  kubectl describe deployment infra-sim-inference-gateway -n ${NS_SIM}"
echo "  kubectl logs -f deployment/gaie-sim-epp -n ${NS_SIM}"
echo ""
echo "To stop the operator: kill ${OPERATOR_PID}"
