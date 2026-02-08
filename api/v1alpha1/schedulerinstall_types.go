package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SchedulerInstallSpec defines the desired state of SchedulerInstall
type SchedulerInstallSpec struct {
	// SchedulerNamespace is the namespace where scheduler components run
	SchedulerNamespace string `json:"schedulerNamespace,omitempty"`

	// SimulatorNamespace is the namespace where simulator backends run
	SimulatorNamespace string `json:"simulatorNamespace,omitempty"`

	// ProxyService defines the proxy Service that fronts simulator backends
	ProxyService ProxyServiceConfig `json:"proxyService,omitempty"`

	// EPP (Endpoint Picker) configuration for the scheduler namespace
	EPP *SchedulerEPPConfig `json:"epp,omitempty"`

	// Gateway configuration (Gateway API)
	Gateway *SchedulerGatewayConfig `json:"gateway,omitempty"`

	// Routing configuration (HTTPRoute + ReferenceGrant)
	Routing *SchedulerRoutingConfig `json:"routing,omitempty"`

	// DestinationRule configuration (Istio)
	DestinationRule *LoadBalancingConfig `json:"destinationRule,omitempty"`

	// EnvoyFilter configuration for ext_proc (scoring)
	EnvoyFilter *SchedulerEnvoyFilterConfig `json:"envoyFilter,omitempty"`
}

// ProxyServiceConfig defines the Service that routes to simulator backends
type ProxyServiceConfig struct {
	// Name of the Service
	// +kubebuilder:default="gaie-inference-scheduling-proxy"
	Name string `json:"name,omitempty"`

	// Port exposed by the Service
	// +kubebuilder:default=8200
	Port int32 `json:"port,omitempty"`

	// TargetPort on backend pods
	// +kubebuilder:default=8200
	TargetPort int32 `json:"targetPort,omitempty"`

	// Selector to match simulator backend pods
	Selector map[string]string `json:"selector,omitempty"`
}

// SchedulerEPPConfig defines EPP configuration for the scheduler namespace
type SchedulerEPPConfig struct {
	// Enabled determines if EPP should be deployed
	Enabled bool `json:"enabled,omitempty"`

	// Name of the EPP deployment/service
	// +kubebuilder:default="gaie-inference-scheduling-epp"
	Name string `json:"name,omitempty"`

	// Replicas is the number of EPP pods
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the container image for EPP
	// +kubebuilder:default="ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0"
	Image string `json:"image,omitempty"`

	// Port for the EPP service
	// +kubebuilder:default=9002
	Port int32 `json:"port,omitempty"`

	// Verbosity sets the EPP log verbosity level (maps to --v)
	// +kubebuilder:default=1
	Verbosity int32 `json:"verbosity,omitempty"`

	// Args are additional arguments to pass to the EPP container
	Args []string `json:"args,omitempty"`

	// PoolName is the InferencePool name EPP watches
	// +kubebuilder:default="gaie-inference-scheduling"
	PoolName string `json:"poolName,omitempty"`

	// PoolNamespace is the namespace of the InferencePool
	PoolNamespace string `json:"poolNamespace,omitempty"`

	// Resources defines the resource requirements for EPP pods
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ConfigProfile selects the generated EPP plugin profile.
	// "default" keeps the existing decode-oriented behavior.
	// "proxy-performance" enables active-request based scoring for proxy endpoints.
	// "proxy-performance-by-backend" adds label-based backend partitioning before scoring.
	// +kubebuilder:validation:Enum=default;proxy-performance;proxy-performance-by-backend
	// +kubebuilder:default="default"
	ConfigProfile string `json:"configProfile,omitempty"`
}

// SchedulerGatewayConfig defines Gateway API Gateway configuration
type SchedulerGatewayConfig struct {
	// Enabled determines if the Gateway should be created
	Enabled bool `json:"enabled,omitempty"`

	// Name of the Gateway
	// +kubebuilder:default="infra-inference-scheduling-inference-gateway"
	Name string `json:"name,omitempty"`

	// ClassName is the GatewayClass name
	// +kubebuilder:default="istio"
	ClassName string `json:"className,omitempty"`

	// ListenerPort is the Gateway listener port
	// +kubebuilder:default=80
	ListenerPort int32 `json:"listenerPort,omitempty"`

	// ListenerProtocol is the Gateway listener protocol
	// +kubebuilder:default="HTTP"
	ListenerProtocol string `json:"listenerProtocol,omitempty"`
}

// GatewayRef identifies a Gateway resource
type GatewayRef struct {
	// Name of the Gateway
	Name string `json:"name,omitempty"`

	// Namespace of the Gateway
	Namespace string `json:"namespace,omitempty"`
}

// SchedulerEnvoyFilterConfig defines EnvoyFilter configuration for ext_proc
type SchedulerEnvoyFilterConfig struct {
	// Enabled determines if the EnvoyFilter should be created
	Enabled bool `json:"enabled,omitempty"`

	// Name of the EnvoyFilter
	// +kubebuilder:default="epp-ext-proc"
	Name string `json:"name,omitempty"`

	// WorkloadSelector labels to match the Gateway pod
	// Defaults to matching the gateway name if not provided
	WorkloadSelector map[string]string `json:"workloadSelector,omitempty"`
}

// SchedulerRoutingConfig defines HTTPRoute + ReferenceGrant configuration
type SchedulerRoutingConfig struct {
	// Enabled determines if routing resources should be created
	Enabled bool `json:"enabled,omitempty"`

	// BackendType selects the HTTPRoute backend ("Service" or "InferencePool")
	// +kubebuilder:default="Service"
	// +kubebuilder:validation:Enum=Service;InferencePool
	BackendType string `json:"backendType,omitempty"`

	// InferencePool references an InferencePool backend when BackendType=InferencePool
	InferencePool *InferencePoolRef `json:"inferencePool,omitempty"`

	// HTTPRouteName is the name of the HTTPRoute
	// +kubebuilder:default="llm-d-inference-scheduling"
	HTTPRouteName string `json:"httpRouteName,omitempty"`

	// ParentGateway references the Gateway for the HTTPRoute
	ParentGateway GatewayRef `json:"parentGateway,omitempty"`
}

// InferencePoolRef identifies an InferencePool backend
type InferencePoolRef struct {
	// Name of the InferencePool
	Name string `json:"name,omitempty"`

	// Namespace of the InferencePool
	Namespace string `json:"namespace,omitempty"`

	// Port to use for the backendRef, if required by the API
	Port int32 `json:"port,omitempty"`
}

// SchedulerInstallStatus defines the observed state of SchedulerInstall
type SchedulerInstallStatus struct {
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=schedinst

// SchedulerInstall is the Schema for the schedulerinstalls API
type SchedulerInstall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchedulerInstallSpec   `json:"spec,omitempty"`
	Status SchedulerInstallStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchedulerInstallList contains a list of SchedulerInstall
type SchedulerInstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchedulerInstall `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SchedulerInstall{}, &SchedulerInstallList{})
}
