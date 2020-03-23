/*
Copyright 2019 dsgkinfo.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	APPNameLabel    = "app.dsgkinfo.com/appName"
	ModuleNameLabel = "app.dsgkinfo.com/moduleName"
	DeploymentType  = "app.dsgkinfo.com/deploymentType"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	DisplayName string   `json:"displayName,omitempty"`
	Description string   `json:"description"`
	UserID      int      `json:"userID"`
	Modules     []Module `json:"modules,omitempty"`
}

type Module struct {
	Name           string            `json:"name"`
	AccessMode     string            `json:"accessMode,omitempty"`
	Proxies        []Proxy           `json:"proxies,omitempty"`
	ServiceConfigs []ServiceConfig   `json:"serviceConfigs,omitempty"`
	AppPkgID       string            `json:"appPkgID,omitempty"`
	Template       v1.DeploymentSpec `json:"template"`
}

type ServiceConfig struct {
	ConfigGroup string `json:"configGroup,omitempty"`
	ConfigItem  string `json:"configItem,omitempty"`
	MountPath   string `json:"mountPath,omitempty"`
}

// 服务出口代理设置,指定协议和内外部端口,自动调谐ingress tcp/udp configmap
type Proxy struct {
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
}

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct {
	TotalModuleNumber    int32  `json:"totalModuleNumber,omitempty"`
	RunningModuleNumber  int32  `json:"runningModuleNumber,omitempty"`
	StartingModuleNumber int32  `json:"startingModuleNumber,omitempty"`
	StoppedModuleNumber  int32  `json:"stoppedModuleNumber,omitempty"`
	RollingUpdateNumber  int32  `json:"rollingUpdateNumber,omitempty"`
	Status               string `json:"status,omitempty"` // 应用状态 {Running| Stopped}
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=app
// +kubebuilder:printcolumn:name="DISPLAYNAME",JSONPath=".spec.displayName",type="string"
// +kubebuilder:printcolumn:name="STATUS",JSONPath=".status.status",type="string"
// +kubebuilder:printcolumn:name="TOTAL",JSONPath=".status.totalModuleNumber",type="string"
// +kubebuilder:printcolumn:name="RUNNING",JSONPath=".status.runningModuleNumber",type="string"
// +kubebuilder:printcolumn:name="ROLLINGUPDATE",JSONPath=".status.rollingUpdateNumber",type="string"

// Application is the Schema for the applications API
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
