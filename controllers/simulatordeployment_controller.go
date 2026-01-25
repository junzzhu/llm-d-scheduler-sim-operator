package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	simv1alpha1 "github.com/llm-d/llm-d-scheduler-sim-operator/api/v1alpha1"
)

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// SimulatorDeploymentReconciler reconciles a SimulatorDeployment object
type SimulatorDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sim.llm-d.io,resources=simulatordeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sim.llm-d.io,resources=simulatordeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sim.llm-d.io,resources=simulatordeployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=destinationrules,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *SimulatorDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the SimulatorDeployment instance
	simDep := &simv1alpha1.SimulatorDeployment{}
	err := r.Get(ctx, req.NamespacedName, simDep)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("SimulatorDeployment resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SimulatorDeployment")
		return ctrl.Result{}, err
	}

	// Set defaults
	r.setDefaults(simDep)

	// Reconcile EPP if enabled
	if simDep.Spec.EPP != nil && simDep.Spec.EPP.Enabled {
		if err := r.reconcileEPP(ctx, simDep); err != nil {
			logger.Error(err, "Failed to reconcile EPP")
			return ctrl.Result{}, err
		}
	}

	// Reconcile Inference Gateways if enabled
	if simDep.Spec.InferenceGateway != nil && simDep.Spec.InferenceGateway.Enabled {
		if err := r.reconcileInferenceGateways(ctx, simDep); err != nil {
			logger.Error(err, "Failed to reconcile Inference Gateways")
			return ctrl.Result{}, err
		}
	}

	// Reconcile Prefill stage if enabled
	if simDep.Spec.Prefill != nil && simDep.Spec.Prefill.Enabled {
		if err := r.reconcilePrefillStage(ctx, simDep); err != nil {
			logger.Error(err, "Failed to reconcile Prefill stage")
			return ctrl.Result{}, err
		}
	}

	// Reconcile Decode stage if enabled
	if simDep.Spec.Decode != nil && simDep.Spec.Decode.Enabled {
		if err := r.reconcileDecodeStage(ctx, simDep); err != nil {
			logger.Error(err, "Failed to reconcile Decode stage")
			return ctrl.Result{}, err
		}
	}

	// Legacy: Reconcile single Deployment (for backward compatibility)
	if (simDep.Spec.Prefill == nil || !simDep.Spec.Prefill.Enabled) &&
		(simDep.Spec.Decode == nil || !simDep.Spec.Decode.Enabled) {
		if err := r.reconcileDeployment(ctx, simDep); err != nil {
			logger.Error(err, "Failed to reconcile Deployment")
			return ctrl.Result{}, err
		}

		// Reconcile Service
		if err := r.reconcileService(ctx, simDep); err != nil {
			logger.Error(err, "Failed to reconcile Service")
			return ctrl.Result{}, err
		}
	}

	// Reconcile DestinationRule if load balancing is enabled
	if simDep.Spec.LoadBalancing != nil && simDep.Spec.LoadBalancing.Enabled {
		if err := r.reconcileDestinationRule(ctx, simDep); err != nil {
			logger.Error(err, "Failed to reconcile DestinationRule")
			return ctrl.Result{}, err
		}
	}

	// Update status
	if err := r.updateStatus(ctx, simDep); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SimulatorDeploymentReconciler) setDefaults(simDep *simv1alpha1.SimulatorDeployment) {
	if simDep.Spec.Replicas == 0 {
		simDep.Spec.Replicas = 2
	}
	if simDep.Spec.Image == "" {
		simDep.Spec.Image = "docker.io/library/llm-d-simulator:local"
	}
	if simDep.Spec.Service.Name == "" {
		simDep.Spec.Service.Name = "gaie-inference-scheduling-proxy"
	}
	if simDep.Spec.Service.Port == 0 {
		simDep.Spec.Service.Port = 8200
	}
	if simDep.Spec.Service.Type == "" {
		simDep.Spec.Service.Type = corev1.ServiceTypeClusterIP
	}
}

func (r *SimulatorDeploymentReconciler) reconcileDeployment(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ms-sim-%s-decode", simDep.Name),
			Namespace: simDep.Namespace,
			Labels: map[string]string{
				"llm-d.ai/role":             "decode",
				"llm-d.ai/inferenceServing": "true",
				"app.kubernetes.io/name":    simDep.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &simDep.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"llm-d.ai/role":          "decode",
					"app.kubernetes.io/name": simDep.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"llm-d.ai/role":             "decode",
						"llm-d.ai/inferenceServing": "true",
						"app.kubernetes.io/name":    simDep.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "decode",
							Image:           simDep.Spec.Image,
							ImagePullPolicy: corev1.PullNever, // Use local image only
							Args: []string{
								"--model", "random",
								"--mode", "random",
								"--port", fmt.Sprintf("%d", simDep.Spec.Service.Port),
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: simDep.Spec.Service.Port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: simDep.Spec.Resources,
						},
					},
				},
			},
		},
	}

	// Set SimulatorDeployment instance as the owner
	if err := controllerutil.SetControllerReference(simDep, deployment, r.Scheme); err != nil {
		return err
	}

	// Check if deployment exists
	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, deployment)
	} else if err != nil {
		return err
	}

	// Update if needed
	if found.Spec.Replicas == nil || *found.Spec.Replicas != simDep.Spec.Replicas {
		found.Spec.Replicas = &simDep.Spec.Replicas
		return r.Update(ctx, found)
	}

	return nil
}

func (r *SimulatorDeploymentReconciler) reconcileService(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      simDep.Spec.Service.Name,
			Namespace: simDep.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": simDep.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: simDep.Spec.Service.Type,
			Selector: map[string]string{
				"llm-d.ai/role":          "decode",
				"app.kubernetes.io/name": simDep.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       simDep.Spec.Service.Port,
					TargetPort: intstr.FromInt(int(simDep.Spec.Service.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Set SimulatorDeployment instance as the owner
	if err := controllerutil.SetControllerReference(simDep, service, r.Scheme); err != nil {
		return err
	}

	// Check if service exists
	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, service)
	} else if err != nil {
		return err
	}

	return nil
}

func (r *SimulatorDeploymentReconciler) reconcileDestinationRule(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	// This would create an Istio DestinationRule
	// For now, we'll skip the actual implementation as it requires Istio CRDs
	// In a real implementation, you would use unstructured.Unstructured to create the DestinationRule
	return nil
}

func (r *SimulatorDeploymentReconciler) updateStatus(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	// Determine which deployment to check based on configuration
	var deploymentName string
	if simDep.Spec.Decode != nil && simDep.Spec.Decode.Enabled {
		// New stage-based deployment
		deploymentName = "ms-sim-llm-d-modelservice-decode"
	} else {
		// Legacy deployment
		deploymentName = fmt.Sprintf("ms-sim-%s-decode", simDep.Name)
	}

	// Get the deployment to check status
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      deploymentName,
		Namespace: simDep.Namespace,
	}, deployment)
	if err != nil {
		// If deployment not found, it might not be created yet - don't fail
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Calculate new status
	newReplicas := deployment.Status.Replicas
	newReadyReplicas := deployment.Status.ReadyReplicas

	// Create condition
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "DeploymentReady",
		Message:            "Simulator deployment is ready",
		LastTransitionTime: metav1.Now(),
	}
	if deployment.Status.ReadyReplicas != deployment.Status.Replicas {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "DeploymentNotReady"
		condition.Message = "Waiting for pods to be ready"
	}

	// Refetch the latest SimulatorDeployment to avoid conflict
	latestSimDep := &simv1alpha1.SimulatorDeployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: simDep.Name, Namespace: simDep.Namespace}, latestSimDep); err != nil {
		return err
	}

	// Update status on the latest object
	latestSimDep.Status.Replicas = newReplicas
	latestSimDep.Status.ReadyReplicas = newReadyReplicas

	// Update or append condition
	found := false
	for i, c := range latestSimDep.Status.Conditions {
		if c.Type == "Ready" {
			latestSimDep.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		latestSimDep.Status.Conditions = append(latestSimDep.Status.Conditions, condition)
	}

	return r.Status().Update(ctx, latestSimDep)
}

func (r *SimulatorDeploymentReconciler) reconcileEPPConfigMap(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	// Default plugins configuration for EPP
	pluginsConfig := `apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
- type: load-aware-scorer
- type: prefix-cache-scorer
  parameters:
    hashBlockSize: 5
    maxPrefixBlocksToMatch: 256
    lruCapacityPerServer: 31250
- type: kv-cache-utilization-scorer
- type: decode-filter
- type: max-score-picker
- type: single-profile-handler
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: decode-filter
  - pluginRef: max-score-picker
  - pluginRef: load-aware-scorer
    weight: 1
  - pluginRef: prefix-cache-scorer
    weight: 2
  - pluginRef: kv-cache-utilization-scorer
    weight: 1
`

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gaie-sim-epp",
			Namespace: simDep.Namespace,
			Labels: map[string]string{
				"llm-d.ai/component":     "epp",
				"app.kubernetes.io/name": simDep.Name,
			},
		},
		Data: map[string]string{
			"default-plugins.yaml": pluginsConfig,
		},
	}

	if err := controllerutil.SetControllerReference(simDep, configMap, r.Scheme); err != nil {
		return err
	}

	found := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, configMap)
	}
	return err
}

func (r *SimulatorDeploymentReconciler) reconcileEPP(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	eppConfig := simDep.Spec.EPP
	if eppConfig == nil {
		return nil
	}

	// Create ConfigMap first
	if err := r.reconcileEPPConfigMap(ctx, simDep); err != nil {
		return err
	}

	// Set defaults
	if eppConfig.Replicas == 0 {
		eppConfig.Replicas = 1
	}
	if eppConfig.Image == "" {
		eppConfig.Image = "ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0"
	}
	if eppConfig.Port == 0 {
		eppConfig.Port = 8100
	}

	// Create EPP Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gaie-sim-epp",
			Namespace: simDep.Namespace,
			Labels: map[string]string{
				"llm-d.ai/component":     "epp",
				"app.kubernetes.io/name": simDep.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &eppConfig.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"llm-d.ai/component":     "epp",
					"app.kubernetes.io/name": simDep.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"llm-d.ai/component":     "epp",
						"app.kubernetes.io/name": simDep.Name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "gaie-sim-epp",
					Containers: []corev1.Container{
						{
							Name:            "epp",
							Image:           eppConfig.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--pool-name",
								"gaie-sim",
								"--pool-namespace",
								simDep.Namespace,
								"--pool-group",
								"inference.networking.x-k8s.io",
								"--zap-encoder",
								"json",
								"--config-file",
								"/config/default-plugins.yaml",
								"--kv-cache-usage-percentage-metric",
								"vllm:kv_cache_usage_perc",
								"--v",
								"1",
								"--tracing=false",
							},
							Env: []corev1.EnvVar{
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: 9002,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "grpc-health",
									ContainerPort: 9003,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "metrics",
									ContainerPort: 9090,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(9003),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								TimeoutSeconds:      1,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(9003),
									},
								},
								PeriodSeconds:    2,
								TimeoutSeconds:   1,
								SuccessThreshold: 1,
								FailureThreshold: 3,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugins-config-volume",
									MountPath: "/config",
								},
							},
							Resources: eppConfig.Resources,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "plugins-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "gaie-sim-epp",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(simDep, deployment, r.Scheme); err != nil {
		return err
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, deployment); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Create EPP Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gaie-sim-epp",
			Namespace: simDep.Namespace,
			Labels: map[string]string{
				"llm-d.ai/component":     "epp",
				"app.kubernetes.io/name": simDep.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"llm-d.ai/component":     "epp",
				"app.kubernetes.io/name": simDep.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       eppConfig.Port,
					TargetPort: intstr.FromInt(int(eppConfig.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(simDep, service, r.Scheme); err != nil {
		return err
	}

	foundSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, service)
	}
	return err
}

func (r *SimulatorDeploymentReconciler) reconcileInferenceGateways(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	gwConfig := simDep.Spec.InferenceGateway
	if gwConfig == nil {
		return nil
	}

	// Reconcile standard gateway
	if gwConfig.Standard != nil && gwConfig.Standard.Enabled {
		if err := r.reconcileGatewayInstance(ctx, simDep, "infra-sim-inference-gateway", gwConfig.Standard, false); err != nil {
			return err
		}
	}

	// Reconcile Istio gateway
	if gwConfig.Istio != nil && gwConfig.Istio.Enabled {
		if err := r.reconcileGatewayInstance(ctx, simDep, "infra-sim-inference-gateway-istio", gwConfig.Istio, true); err != nil {
			return err
		}
	}

	return nil
}

func (r *SimulatorDeploymentReconciler) reconcileGatewayConfigMap(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment, name string) error {
	// Basic Envoy configuration for the gateway
	envoyConfig := `admin:
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 19000

static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 80
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          route_config:
            name: local_route
            virtual_hosts:
            - name: backend
              domains: ["*"]
              routes:
              - match:
                  prefix: "/"
                route:
                  cluster: simulator_cluster
          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router

  clusters:
  - name: simulator_cluster
    connect_timeout: 5s
    type: STRICT_DNS
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: simulator_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: ms-sim-llm-d-modelservice-decode
                port_value: 8200
`

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: simDep.Namespace,
			Labels: map[string]string{
				"llm-d.ai/component":     "gateway",
				"app.kubernetes.io/name": simDep.Name,
			},
		},
		Data: map[string]string{
			"envoy.yaml": envoyConfig,
		},
	}

	if err := controllerutil.SetControllerReference(simDep, configMap, r.Scheme); err != nil {
		return err
	}

	found := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, configMap)
	}
	return err
}

func (r *SimulatorDeploymentReconciler) buildGatewayContainer(config *simv1alpha1.GatewayInstanceConfig, isIstio bool) corev1.Container {
	if isIstio {
		// Istio uses pilot-agent with different args
		return corev1.Container{
			Name:            "istio-proxy",
			Image:           config.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"proxy",
				"sidecar",
			},
			Env: []corev1.EnvVar{
				{
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.name",
						},
					},
				},
				{
					Name: "POD_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.namespace",
						},
					},
				},
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Resources: config.Resources,
		}
	}

	// Standard Envoy gateway
	return corev1.Container{
		Name:            "kgateway-proxy",
		Image:           config.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"--disable-hot-restart",
			"--service-node",
			"$(POD_NAME).$(POD_NAMESPACE)",
			"--log-level",
			"warn",
			"--component-log-level",
			"connection:warn,http:warn,upstream:warn",
		},
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.namespace",
					},
				},
			},
			{
				Name:  "ENVOY_UID",
				Value: "0",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "listener-80",
				ContainerPort: 80,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "http-monitoring",
				ContainerPort: 9091,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/ready",
					Port:   intstr.FromInt(19000),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			TimeoutSeconds:      1,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/ready",
					Port:   intstr.FromInt(19000),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			PeriodSeconds:    1,
			TimeoutSeconds:   2,
			SuccessThreshold: 1,
			FailureThreshold: 60,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "envoy-config",
				MountPath: "/etc/envoy",
			},
		},
		Resources: config.Resources,
	}
}

func (r *SimulatorDeploymentReconciler) buildGatewayVolumes(name string, isIstio bool) []corev1.Volume {
	if isIstio {
		// Istio doesn't need ConfigMap volume
		return []corev1.Volume{}
	}

	// Standard Envoy gateway needs ConfigMap
	return []corev1.Volume{
		{
			Name: "envoy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
				},
			},
		},
	}
}

func (r *SimulatorDeploymentReconciler) reconcileGatewayInstance(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment, name string, config *simv1alpha1.GatewayInstanceConfig, isIstio bool) error {
	// Set defaults
	if config.Replicas == 0 {
		config.Replicas = 1
	}
	if config.Image == "" {
		if isIstio {
			config.Image = "docker.io/istio/proxyv2:1.28.1"
		} else {
			config.Image = "cr.kgateway.dev/kgateway-dev/envoy-wrapper:v2.1.1"
		}
	}
	if config.Port == 0 {
		config.Port = 8080
	}

	// Create ConfigMap for Envoy configuration
	if err := r.reconcileGatewayConfigMap(ctx, simDep, name); err != nil {
		return err
	}

	labels := map[string]string{
		"llm-d.ai/component":     "gateway",
		"app.kubernetes.io/name": simDep.Name,
	}

	annotations := map[string]string{}
	if isIstio {
		labels["llm-d.ai/gateway-type"] = "istio"
		// Prevent sidecar injection since we're manually defining the proxy
		annotations["sidecar.istio.io/inject"] = "false"
	}

	// Create Gateway Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: simDep.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &config.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						r.buildGatewayContainer(config, isIstio),
					},
					Volumes: r.buildGatewayVolumes(name, isIstio),
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(simDep, deployment, r.Scheme); err != nil {
		return err
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, deployment); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Create Gateway Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: simDep.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       config.Port,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(simDep, service, r.Scheme); err != nil {
		return err
	}

	foundSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, service)
	}
	return err
}

func (r *SimulatorDeploymentReconciler) reconcilePrefillStage(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	return r.reconcileStage(ctx, simDep, "prefill", simDep.Spec.Prefill)
}

func (r *SimulatorDeploymentReconciler) reconcileDecodeStage(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment) error {
	return r.reconcileStage(ctx, simDep, "decode", simDep.Spec.Decode)
}

func (r *SimulatorDeploymentReconciler) reconcileStage(ctx context.Context, simDep *simv1alpha1.SimulatorDeployment, stage string, config *simv1alpha1.StageConfig) error {
	if config == nil {
		return nil
	}

	// Set defaults
	if config.Replicas == 0 {
		config.Replicas = 2
	}
	if config.Image == "" {
		config.Image = simDep.Spec.Image
		if config.Image == "" {
			config.Image = "docker.io/library/llm-d-simulator:local"
		}
	}
	if config.Port == 0 {
		config.Port = 8200
	}

	deploymentName := fmt.Sprintf("ms-sim-llm-d-modelservice-%s", stage)
	labels := map[string]string{
		"llm-d.ai/role":             stage,
		"llm-d.ai/inferenceServing": "true",
		"app.kubernetes.io/name":    simDep.Name,
	}

	// Build container args
	args := []string{
		"--model", "random",
		"--mode", "random",
		"--port", fmt.Sprintf("%d", config.Port),
	}
	if len(config.Args) > 0 {
		args = append(args, config.Args...)
	}

	// Create Stage Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: simDep.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &config.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            stage,
							Image:           config.Image,
							ImagePullPolicy: corev1.PullNever,
							Args:            args,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: config.Port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: config.Resources,
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(simDep, deployment, r.Scheme); err != nil {
		return err
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, deployment); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// Update if replicas changed
		if found.Spec.Replicas == nil || *found.Spec.Replicas != config.Replicas {
			found.Spec.Replicas = &config.Replicas
			if err := r.Update(ctx, found); err != nil {
				return err
			}
		}
	}

	// Create Stage Service
	serviceName := fmt.Sprintf("ms-sim-llm-d-modelservice-%s", stage)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: simDep.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       config.Port,
					TargetPort: intstr.FromInt(int(config.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(simDep, service, r.Scheme); err != nil {
		return err
	}

	foundSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, service)
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimulatorDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&simv1alpha1.SimulatorDeployment{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
