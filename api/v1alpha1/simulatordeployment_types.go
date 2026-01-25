package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SimulatorDeploymentSpec defines the desired state of SimulatorDeployment
type SimulatorDeploymentSpec struct {
	// Replicas is the number of simulator pods to run (deprecated, use Prefill/Decode replicas)
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the container image for the simulator
	// +kubebuilder:default="ghcr.io/llm-d/llm-d-simulator:latest"
	Image string `json:"image,omitempty"`

	// LogVerbosity sets klog verbosity level for simulator pods
	// +kubebuilder:default=5
	LogVerbosity int32 `json:"logVerbosity,omitempty"`

	// Resources defines the resource requirements for simulator pods
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Service configuration
	Service ServiceConfig `json:"service,omitempty"`

	// Gateway configuration
	Gateway GatewayConfig `json:"gateway,omitempty"`

	// LoadBalancing configuration (optional)
	LoadBalancing *LoadBalancingConfig `json:"loadBalancing,omitempty"`

	// EPP (Endpoint Picker) configuration
	EPP *EPPConfig `json:"epp,omitempty"`

	// Prefill stage configuration
	Prefill *StageConfig `json:"prefill,omitempty"`

	// Decode stage configuration
	Decode *StageConfig `json:"decode,omitempty"`

	// InferenceGateway configuration
	InferenceGateway *InferenceGatewayConfig `json:"inferenceGateway,omitempty"`
}

// ServiceConfig defines service configuration
type ServiceConfig struct {
	// Name of the service
	// +kubebuilder:default="gaie-inference-scheduling-proxy"
	Name string `json:"name,omitempty"`

	// Port for the service
	// +kubebuilder:default=8200
	Port int32 `json:"port,omitempty"`

	// Type of service (ClusterIP, LoadBalancer, NodePort)
	// +kubebuilder:default="ClusterIP"
	Type corev1.ServiceType `json:"type,omitempty"`
}

// GatewayConfig defines gateway configuration
type GatewayConfig struct {
	// Enabled determines if gateway should be created
	Enabled bool `json:"enabled,omitempty"`

	// ClassName is the gateway class (istio, kgateway, etc.)
	// +kubebuilder:default="istio"
	ClassName string `json:"className,omitempty"`

	// RouteName is the name for the route/gateway
	// +kubebuilder:default="infra-sim-inference-gateway"
	RouteName string `json:"routeName,omitempty"`
}

// LoadBalancingConfig defines load balancing configuration
type LoadBalancingConfig struct {
	// Enabled determines if custom load balancing should be configured
	Enabled bool `json:"enabled,omitempty"`

	// Algorithm specifies the load balancing algorithm
	// +kubebuilder:validation:Enum=ROUND_ROBIN;LEAST_REQUEST;RANDOM;LEAST_CONN
	// +kubebuilder:default="ROUND_ROBIN"
	Algorithm string `json:"algorithm,omitempty"`

	// ConnectionPool settings
	ConnectionPool *ConnectionPoolConfig `json:"connectionPool,omitempty"`
}

// ConnectionPoolConfig defines connection pool settings
type ConnectionPoolConfig struct {
	// HTTP1MaxPendingRequests
	// +kubebuilder:default=1
	HTTP1MaxPendingRequests int32 `json:"http1MaxPendingRequests,omitempty"`

	// MaxRequestsPerConnection
	// +kubebuilder:default=1
	MaxRequestsPerConnection int32 `json:"maxRequestsPerConnection,omitempty"`
}

// EPPConfig defines EPP (Endpoint Picker) configuration
type EPPConfig struct {
	// Enabled determines if EPP should be deployed
	Enabled bool `json:"enabled,omitempty"`

	// Replicas is the number of EPP pods
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the container image for EPP
	// +kubebuilder:default="ghcr.io/llm-d/llm-d-inference-scheduler:v0.4.0"
	Image string `json:"image,omitempty"`

	// Port for the EPP service
	// +kubebuilder:default=8100
	Port int32 `json:"port,omitempty"`

	// Resources defines the resource requirements for EPP pods
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// StageConfig defines configuration for prefill or decode stage
type StageConfig struct {
	// Enabled determines if this stage should be deployed
	Enabled bool `json:"enabled,omitempty"`

	// Replicas is the number of pods for this stage
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the container image for this stage
	Image string `json:"image,omitempty"`

	// Port for the service
	// +kubebuilder:default=8200
	Port int32 `json:"port,omitempty"`

	// LogVerbosity sets klog verbosity level for this stage
	// +kubebuilder:default=5
	LogVerbosity int32 `json:"logVerbosity,omitempty"`

	// Resources defines the resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Args are additional arguments to pass to the container
	Args []string `json:"args,omitempty"`
}

// InferenceGatewayConfig defines inference gateway configuration
type InferenceGatewayConfig struct {
	// Enabled determines if inference gateways should be deployed
	Enabled bool `json:"enabled,omitempty"`

	// Standard gateway configuration
	Standard *GatewayInstanceConfig `json:"standard,omitempty"`

	// Istio gateway configuration
	Istio *GatewayInstanceConfig `json:"istio,omitempty"`
}

// GatewayInstanceConfig defines configuration for a gateway instance
type GatewayInstanceConfig struct {
	// Enabled determines if this gateway instance should be deployed
	Enabled bool `json:"enabled,omitempty"`

	// Replicas is the number of gateway pods
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the container image for the gateway
	// Standard gateway default: cr.kgateway.dev/kgateway-dev/envoy-wrapper:v2.1.1
	// Istio gateway default: docker.io/istio/proxyv2:1.28.1
	Image string `json:"image,omitempty"`

	// Port for the gateway service
	// +kubebuilder:default=8080
	Port int32 `json:"port,omitempty"`

	// Resources defines the resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// SimulatorDeploymentStatus defines the observed state of SimulatorDeployment
type SimulatorDeploymentStatus struct {
	// Replicas is the current number of replicas
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Endpoints lists the service endpoints
	Endpoints []string `json:"endpoints,omitempty"`

	// GatewayURL is the external URL for the gateway
	GatewayURL string `json:"gatewayURL,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=simdep
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SimulatorDeployment is the Schema for the simulatordeployments API
type SimulatorDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SimulatorDeploymentSpec   `json:"spec,omitempty"`
	Status SimulatorDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SimulatorDeploymentList contains a list of SimulatorDeployment
type SimulatorDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SimulatorDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SimulatorDeployment{}, &SimulatorDeploymentList{})
}
