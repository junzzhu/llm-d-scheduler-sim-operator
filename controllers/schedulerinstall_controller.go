package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	simv1alpha1 "github.com/llm-d/llm-d-scheduler-sim-operator/api/v1alpha1"
)

// SchedulerInstallReconciler reconciles a SchedulerInstall object
type SchedulerInstallReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RESTMapper meta.RESTMapper
}

//+kubebuilder:rbac:groups=sim.llm-d.io,resources=schedulerinstalls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sim.llm-d.io,resources=schedulerinstalls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sim.llm-d.io,resources=schedulerinstalls/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways;httproutes;referencegrants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.istio.io,resources=destinationrules,verbs=get;list;watch;create;update;patch;delete

func (r *SchedulerInstallReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	install := &simv1alpha1.SchedulerInstall{}
	if err := r.Get(ctx, req.NamespacedName, install); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	r.setDefaults(install)
	if install.Spec.SimulatorNamespace == "" {
		logger.Error(fmt.Errorf("spec.simulatorNamespace is required"), "invalid SchedulerInstall")
		return ctrl.Result{}, fmt.Errorf("spec.simulatorNamespace is required")
	}

	if install.Spec.EPP != nil && install.Spec.EPP.Enabled {
		if err := r.reconcileSchedulerEPP(ctx, install); err != nil {
			return ctrl.Result{}, err
		}
	}

	if install.Spec.Gateway != nil && install.Spec.Gateway.Enabled {
		if err := r.reconcileGateway(ctx, install); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.reconcileProxyService(ctx, install); err != nil {
		return ctrl.Result{}, err
	}

	if install.Spec.Routing != nil && install.Spec.Routing.Enabled {
		if err := r.reconcileReferenceGrant(ctx, install); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileHTTPRoute(ctx, install); err != nil {
			return ctrl.Result{}, err
		}
	}

	if install.Spec.DestinationRule != nil && install.Spec.DestinationRule.Enabled {
		if err := r.reconcileDestinationRule(ctx, install); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.updateStatus(ctx, install, true, "Reconciled", "SchedulerInstall resources are ready"); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SchedulerInstallReconciler) setDefaults(install *simv1alpha1.SchedulerInstall) {
	if install.Spec.SchedulerNamespace == "" {
		install.Spec.SchedulerNamespace = install.Namespace
	}
	if install.Spec.ProxyService.Name == "" {
		install.Spec.ProxyService.Name = "gaie-inference-scheduling-proxy"
	}
	if install.Spec.ProxyService.Port == 0 {
		install.Spec.ProxyService.Port = 8200
	}
	if install.Spec.ProxyService.TargetPort == 0 {
		install.Spec.ProxyService.TargetPort = install.Spec.ProxyService.Port
	}
	if len(install.Spec.ProxyService.Selector) == 0 {
		install.Spec.ProxyService.Selector = map[string]string{
			"llm-d.ai/role":             "decode",
			"llm-d.ai/inferenceServing": "true",
		}
	}

	if install.Spec.EPP != nil {
		if install.Spec.EPP.Name == "" {
			install.Spec.EPP.Name = "gaie-inference-scheduling-epp"
		}
		if install.Spec.EPP.Replicas == 0 {
			install.Spec.EPP.Replicas = 1
		}
		if install.Spec.EPP.Image == "" {
			install.Spec.EPP.Image = "ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0"
		}
		if install.Spec.EPP.Port == 0 {
			install.Spec.EPP.Port = 9002
		}
		if install.Spec.EPP.PoolName == "" {
			install.Spec.EPP.PoolName = "gaie-inference-scheduling"
		}
		if install.Spec.EPP.PoolNamespace == "" {
			install.Spec.EPP.PoolNamespace = install.Spec.SimulatorNamespace
		}
	}

	if install.Spec.Gateway != nil {
		if install.Spec.Gateway.Name == "" {
			install.Spec.Gateway.Name = "infra-inference-scheduling-inference-gateway"
		}
		if install.Spec.Gateway.ClassName == "" {
			install.Spec.Gateway.ClassName = "istio"
		}
		if install.Spec.Gateway.ListenerPort == 0 {
			install.Spec.Gateway.ListenerPort = 80
		}
		if install.Spec.Gateway.ListenerProtocol == "" {
			install.Spec.Gateway.ListenerProtocol = "HTTP"
		}
	}

	if install.Spec.Routing != nil {
		if install.Spec.Routing.HTTPRouteName == "" {
			install.Spec.Routing.HTTPRouteName = "llm-d-inference-scheduling"
		}
		if install.Spec.Routing.ParentGateway.Name == "" && install.Spec.Gateway != nil {
			install.Spec.Routing.ParentGateway.Name = install.Spec.Gateway.Name
		}
		if install.Spec.Routing.ParentGateway.Namespace == "" {
			install.Spec.Routing.ParentGateway.Namespace = install.Spec.SchedulerNamespace
		}
	}

	if install.Spec.DestinationRule != nil {
		if install.Spec.DestinationRule.Algorithm == "" {
			install.Spec.DestinationRule.Algorithm = "ROUND_ROBIN"
		}
		if install.Spec.DestinationRule.ConnectionPool == nil {
			install.Spec.DestinationRule.ConnectionPool = &simv1alpha1.ConnectionPoolConfig{
				HTTP1MaxPendingRequests: 1,
				MaxRequestsPerConnection: 1,
			}
		}
	}
}

func (r *SchedulerInstallReconciler) reconcileSchedulerEPP(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	epp := install.Spec.EPP
	if epp == nil {
		return nil
	}

	if err := r.reconcileEPPServiceAccount(ctx, install); err != nil {
		return err
	}
	if err := r.reconcileEPPRBAC(ctx, install); err != nil {
		return err
	}
	if err := r.reconcileEPPConfigMap(ctx, install); err != nil {
		return err
	}
	if err := r.reconcileEPPDeployment(ctx, install); err != nil {
		return err
	}
	return r.reconcileEPPService(ctx, install)
}

func (r *SchedulerInstallReconciler) reconcileEPPServiceAccount(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      install.Spec.EPP.Name,
			Namespace: install.Spec.SchedulerNamespace,
		},
	}
	if err := controllerutil.SetControllerReference(install, sa, r.Scheme); err != nil {
		return err
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sa, func() error { return nil })
	return err
}

func (r *SchedulerInstallReconciler) reconcileEPPRBAC(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      install.Spec.EPP.Name,
			Namespace: install.Spec.SimulatorNamespace,
			Labels:    r.crossNamespaceLabels(install),
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "endpoints"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"inference.networking.x-k8s.io"},
				Resources: []string{"inferencepools", "inferenceobjectives"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
		}
		return nil
	})
	if err != nil {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      install.Spec.EPP.Name,
			Namespace: install.Spec.SimulatorNamespace,
			Labels:    r.crossNamespaceLabels(install),
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     install.Spec.EPP.Name,
		}
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      install.Spec.EPP.Name,
				Namespace: install.Spec.SchedulerNamespace,
			},
		}
		return nil
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileEPPConfigMap(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	configName := fmt.Sprintf("%s-config", install.Spec.EPP.Name)
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
			Name:      configName,
			Namespace: install.Spec.SchedulerNamespace,
		},
	}
	if err := controllerutil.SetControllerReference(install, configMap, r.Scheme); err != nil {
		return err
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		if configMap.Labels == nil {
			configMap.Labels = map[string]string{}
		}
		configMap.Labels["app.kubernetes.io/name"] = install.Name
		configMap.Data = map[string]string{
			"epp-config.yaml": pluginsConfig,
		}
		return nil
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileEPPDeployment(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	epp := install.Spec.EPP
	configName := fmt.Sprintf("%s-config", epp.Name)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      epp.Name,
			Namespace: install.Spec.SchedulerNamespace,
		},
	}
	if err := controllerutil.SetControllerReference(install, deployment, r.Scheme); err != nil {
		return err
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		labels := map[string]string{
			"app":                    epp.Name,
			"app.kubernetes.io/name": install.Name,
		}
		deployment.Labels = labels
		deployment.Spec.Replicas = &epp.Replicas
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		deployment.Spec.Template.ObjectMeta.Labels = labels
		deployment.Spec.Template.Spec.ServiceAccountName = epp.Name
		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:            "epp",
				Image:           epp.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args: []string{
					"--pool-name", epp.PoolName,
					"--pool-namespace", epp.PoolNamespace,
					"--pool-group", "inference.networking.x-k8s.io",
					"--zap-encoder", "json",
					"--config-file", "/etc/epp/epp-config.yaml",
					"--kv-cache-usage-percentage-metric", "vllm:kv_cache_usage_perc",
					"--grpc-port", "9002",
					"--grpc-health-port", "9003",
					"--v", "1",
					"--tracing=false",
				},
				Ports: []corev1.ContainerPort{
					{Name: "grpc", ContainerPort: 9002, Protocol: corev1.ProtocolTCP},
					{Name: "grpc-health", ContainerPort: 9003, Protocol: corev1.ProtocolTCP},
					{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(9003)},
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       10,
					TimeoutSeconds:      1,
					SuccessThreshold:    1,
					FailureThreshold:    3,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(9003)},
					},
					PeriodSeconds:    2,
					TimeoutSeconds:   1,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "epp-config",
						MountPath: "/etc/epp",
					},
				},
				Resources: epp.Resources,
			},
		}
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "epp-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configName},
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileEPPService(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	epp := install.Spec.EPP
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      epp.Name,
			Namespace: install.Spec.SchedulerNamespace,
		},
	}
	if err := controllerutil.SetControllerReference(install, service, r.Scheme); err != nil {
		return err
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = map[string]string{"app": epp.Name}
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "grpc",
				Port:       epp.Port,
				TargetPort: intstr.FromInt(9002),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return nil
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileProxyService(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      install.Spec.ProxyService.Name,
			Namespace: install.Spec.SimulatorNamespace,
			Labels:    r.crossNamespaceLabels(install),
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = install.Spec.ProxyService.Selector
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       install.Spec.ProxyService.Port,
				TargetPort: intstr.FromInt(int(install.Spec.ProxyService.TargetPort)),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		return nil
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileGateway(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	gvk := schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "Gateway"}
	if !r.gvkSupported(gvk) {
		return nil
	}

	gateway := &unstructured.Unstructured{}
	gateway.SetGroupVersionKind(gvk)
	gateway.SetName(install.Spec.Gateway.Name)
	gateway.SetNamespace(install.Spec.SchedulerNamespace)
	if err := controllerutil.SetControllerReference(install, gateway, r.Scheme); err != nil {
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, gateway, func() error {
		spec := map[string]interface{}{
			"gatewayClassName": install.Spec.Gateway.ClassName,
			"listeners": []interface{}{
				map[string]interface{}{
					"name":     "default",
					"port":     int64(install.Spec.Gateway.ListenerPort),
					"protocol": install.Spec.Gateway.ListenerProtocol,
				},
			},
		}
		return unstructured.SetNestedField(gateway.Object, spec, "spec")
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileHTTPRoute(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	gvk := schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRoute"}
	if !r.gvkSupported(gvk) {
		return nil
	}

	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(gvk)
	route.SetName(install.Spec.Routing.HTTPRouteName)
	route.SetNamespace(install.Spec.SchedulerNamespace)
	if err := controllerutil.SetControllerReference(install, route, r.Scheme); err != nil {
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
		spec := map[string]interface{}{
			"parentRefs": []interface{}{
				map[string]interface{}{
					"group": "gateway.networking.k8s.io",
					"kind":  "Gateway",
					"name":  install.Spec.Routing.ParentGateway.Name,
					"namespace": install.Spec.Routing.ParentGateway.Namespace,
				},
			},
			"rules": []interface{}{
				map[string]interface{}{
					"matches": []interface{}{
						map[string]interface{}{
							"path": map[string]interface{}{
								"type":  "PathPrefix",
								"value": "/",
							},
						},
					},
					"backendRefs": []interface{}{
						map[string]interface{}{
							"group":     "",
							"kind":      "Service",
							"name":      install.Spec.ProxyService.Name,
							"namespace": install.Spec.SimulatorNamespace,
							"port":      int64(install.Spec.ProxyService.Port),
							"weight":    int64(1),
						},
					},
					"timeouts": map[string]interface{}{
						"backendRequest": "0s",
						"request":        "0s",
					},
				},
			},
		}
		return unstructured.SetNestedField(route.Object, spec, "spec")
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileReferenceGrant(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	gvk := schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1beta1", Kind: "ReferenceGrant"}
	if !r.gvkSupported(gvk) {
		return nil
	}

	grant := &unstructured.Unstructured{}
	grant.SetGroupVersionKind(gvk)
	grant.SetName("allow-scheduler-httproute")
	grant.SetNamespace(install.Spec.SimulatorNamespace)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, grant, func() error {
		if grant.GetLabels() == nil {
			grant.SetLabels(map[string]string{})
		}
		labels := grant.GetLabels()
		for k, v := range r.crossNamespaceLabels(install) {
			labels[k] = v
		}
		grant.SetLabels(labels)
		spec := map[string]interface{}{
			"from": []interface{}{
				map[string]interface{}{
					"group":     "gateway.networking.k8s.io",
					"kind":      "HTTPRoute",
					"namespace": install.Spec.SchedulerNamespace,
				},
			},
			"to": []interface{}{
				map[string]interface{}{
					"group": "inference.networking.k8s.io",
					"kind":  "InferencePool",
				},
				map[string]interface{}{
					"group": "",
					"kind":  "Service",
				},
			},
		}
		return unstructured.SetNestedField(grant.Object, spec, "spec")
	})
	return err
}

func (r *SchedulerInstallReconciler) reconcileDestinationRule(ctx context.Context, install *simv1alpha1.SchedulerInstall) error {
	gvk := schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1beta1", Kind: "DestinationRule"}
	if !r.gvkSupported(gvk) {
		return nil
	}

	dr := &unstructured.Unstructured{}
	dr.SetGroupVersionKind(gvk)
	dr.SetName(fmt.Sprintf("%s-lb", install.Spec.ProxyService.Name))
	dr.SetNamespace(install.Spec.SimulatorNamespace)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dr, func() error {
		if dr.GetLabels() == nil {
			dr.SetLabels(map[string]string{})
		}
		labels := dr.GetLabels()
		for k, v := range r.crossNamespaceLabels(install) {
			labels[k] = v
		}
		dr.SetLabels(labels)

		trafficPolicy := map[string]interface{}{
			"loadBalancer": map[string]interface{}{
				"simple": install.Spec.DestinationRule.Algorithm,
			},
		}
		if install.Spec.DestinationRule.ConnectionPool != nil {
			trafficPolicy["connectionPool"] = map[string]interface{}{
				"http": map[string]interface{}{
					"http1MaxPendingRequests": int64(install.Spec.DestinationRule.ConnectionPool.HTTP1MaxPendingRequests),
					"maxRequestsPerConnection": int64(install.Spec.DestinationRule.ConnectionPool.MaxRequestsPerConnection),
				},
			}
		}
		spec := map[string]interface{}{
			"host": fmt.Sprintf("%s.%s.svc.cluster.local", install.Spec.ProxyService.Name, install.Spec.SimulatorNamespace),
			"trafficPolicy": trafficPolicy,
		}
		return unstructured.SetNestedField(dr.Object, spec, "spec")
	})
	return err
}

func (r *SchedulerInstallReconciler) updateStatus(ctx context.Context, install *simv1alpha1.SchedulerInstall, ready bool, reason, message string) error {
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	if ready {
		condition.Status = metav1.ConditionTrue
	}

	latest := &simv1alpha1.SchedulerInstall{}
	if err := r.Get(ctx, types.NamespacedName{Name: install.Name, Namespace: install.Namespace}, latest); err != nil {
		return err
	}
	found := false
	for i, c := range latest.Status.Conditions {
		if c.Type == "Ready" {
			latest.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		latest.Status.Conditions = append(latest.Status.Conditions, condition)
	}
	return r.Status().Update(ctx, latest)
}

func (r *SchedulerInstallReconciler) gvkSupported(gvk schema.GroupVersionKind) bool {
	if r.RESTMapper == nil {
		return true
	}
	_, err := r.RESTMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	return err == nil
}

func (r *SchedulerInstallReconciler) crossNamespaceLabels(install *simv1alpha1.SchedulerInstall) map[string]string {
	return map[string]string{
		"sim.llm-d.io/schedulerInstall": install.Name,
		"sim.llm-d.io/schedulerNamespace": install.Namespace,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchedulerInstallReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.RESTMapper == nil {
		r.RESTMapper = mgr.GetRESTMapper()
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&simv1alpha1.SchedulerInstall{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Complete(r)
}
