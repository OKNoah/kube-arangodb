//
// DISCLAIMER
//
// Copyright 2018 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//
// Author Ewout Prangsma
//

package v1alpha

import (
	"math"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/arangodb/kube-arangodb/pkg/util"
	arangod_options "github.com/arangodb/kube-arangodb/pkg/util/arangod/options"
	arangosync_options "github.com/arangodb/kube-arangodb/pkg/util/arangosync/options"
	"github.com/arangodb/kube-arangodb/pkg/util/k8sutil"
)

// ServerGroupSpec contains the specification for all servers in a specific group (e.g. all agents)
type ServerGroupSpec struct {
	// Count holds the requested number of servers
	Count *int `json:"count,omitempty"`
	// MinCount specifies a lower limit for count
	MinCount *int `json:"minCount,omitempty"`
	// MaxCount specifies a upper limit for count
	MaxCount *int `json:"maxCount,omitempty"`
	// Args holds additional commandline arguments
	Args []string `json:"args,omitempty"`
	// StorageClassName specifies the classname for storage of the servers.
	StorageClassName *string `json:"storageClassName,omitempty"`
	// Resources holds resource requests & limits
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// Tolerations specifies the tolerations added to Pods in this group.
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// ServiceAccountName specifies the name of the service account used for Pods in this group.
	ServiceAccountName *string `json:"serviceAccountName,omitempty"`
	// NodeSelector speficies a set of selectors for nodes
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// GetCount returns the value of count.
func (s ServerGroupSpec) GetCount() int {
	return util.IntOrDefault(s.Count)
}

// GetMinCount returns MinCount or 1 if not set
func (s ServerGroupSpec) GetMinCount() int {
	return util.IntOrDefault(s.MinCount, 1)
}

// GetMaxCount returns MaxCount or
func (s ServerGroupSpec) GetMaxCount() int {
	return util.IntOrDefault(s.MaxCount, math.MaxInt32)
}

// GetNodeSelector returns the selectors for nodes of this group
func (s ServerGroupSpec) GetNodeSelector() map[string]string {
	return s.NodeSelector
}

// GetArgs returns the value of args.
func (s ServerGroupSpec) GetArgs() []string {
	return s.Args
}

// GetStorageClassName returns the value of storageClassName.
func (s ServerGroupSpec) GetStorageClassName() string {
	return util.StringOrDefault(s.StorageClassName)
}

// GetTolerations returns the value of tolerations.
func (s ServerGroupSpec) GetTolerations() []v1.Toleration {
	return s.Tolerations
}

// GetServiceAccountName returns the value of serviceAccountName.
func (s ServerGroupSpec) GetServiceAccountName() string {
	return util.StringOrDefault(s.ServiceAccountName)
}

// Validate the given group spec
func (s ServerGroupSpec) Validate(group ServerGroup, used bool, mode DeploymentMode, env Environment) error {
	if used {
		minCount := 1
		if env == EnvironmentProduction {
			// Set validation boundaries for production mode
			switch group {
			case ServerGroupSingle:
				if mode == DeploymentModeActiveFailover {
					minCount = 2
				}
			case ServerGroupAgents:
				minCount = 3
			case ServerGroupDBServers, ServerGroupCoordinators, ServerGroupSyncMasters, ServerGroupSyncWorkers:
				minCount = 2
			}
		} else {
			// Set validation boundaries for development mode
			switch group {
			case ServerGroupSingle:
				if mode == DeploymentModeActiveFailover {
					minCount = 2
				}
			case ServerGroupDBServers:
				minCount = 2
			}
		}
		if s.GetMinCount() > s.GetMaxCount() {
			return maskAny(errors.Wrapf(ValidationError, "Invalid min/maxCount. Min (%d) bigger than Max (%d)", s.GetMinCount(), s.GetMaxCount()))
		}
		if s.GetCount() < s.GetMinCount() {
			return maskAny(errors.Wrapf(ValidationError, "Invalid count value %d. Expected >= %d", s.GetCount(), s.GetMinCount()))
		}
		if s.GetCount() > s.GetMaxCount() {
			return maskAny(errors.Wrapf(ValidationError, "Invalid count value %d. Expected <= %d", s.GetCount(), s.GetMaxCount()))
		}
		if s.GetCount() < minCount {
			return maskAny(errors.Wrapf(ValidationError, "Invalid count value %d. Expected >= %d (implicit minimum; by deployment mode)", s.GetCount(), minCount))
		}
		if s.GetCount() > 1 && group == ServerGroupSingle && mode == DeploymentModeSingle {
			return maskAny(errors.Wrapf(ValidationError, "Invalid count value %d. Expected 1", s.GetCount()))
		}
		if name := s.GetServiceAccountName(); name != "" {
			if err := k8sutil.ValidateOptionalResourceName(name); err != nil {
				return maskAny(errors.Wrapf(ValidationError, "Invalid serviceAccountName: %s", err))
			}
		}
		if name := s.GetStorageClassName(); name != "" {
			if err := k8sutil.ValidateOptionalResourceName(name); err != nil {
				return maskAny(errors.Wrapf(ValidationError, "Invalid storageClassName: %s", err))
			}
		}
		for _, arg := range s.Args {
			parts := strings.Split(arg, "=")
			optionKey := strings.TrimSpace(parts[0])
			if group.IsArangod() {
				if arangod_options.IsCriticalOption(optionKey) {
					return maskAny(errors.Wrapf(ValidationError, "Critical option '%s' cannot be overriden", optionKey))
				}
			} else if group.IsArangosync() {
				if arangosync_options.IsCriticalOption(optionKey) {
					return maskAny(errors.Wrapf(ValidationError, "Critical option '%s' cannot be overriden", optionKey))
				}
			}
		}
	} else if s.GetCount() != 0 {
		return maskAny(errors.Wrapf(ValidationError, "Invalid count value %d for un-used group. Expected 0", s.GetCount()))
	}
	return nil
}

// SetDefaults fills in missing defaults
func (s *ServerGroupSpec) SetDefaults(group ServerGroup, used bool, mode DeploymentMode) {
	if s.GetCount() == 0 && used {
		switch group {
		case ServerGroupSingle:
			if mode == DeploymentModeSingle {
				s.Count = util.NewInt(1) // Single server
			} else {
				s.Count = util.NewInt(2) // ActiveFailover
			}
		default:
			s.Count = util.NewInt(3)
		}
	} else if s.GetCount() > 0 && !used {
		s.Count = nil
		s.MinCount = nil
		s.MaxCount = nil
	}
	if _, found := s.Resources.Requests[v1.ResourceStorage]; !found {
		switch group {
		case ServerGroupSingle, ServerGroupAgents, ServerGroupDBServers:
			if s.Resources.Requests == nil {
				s.Resources.Requests = make(map[v1.ResourceName]resource.Quantity)
			}
			s.Resources.Requests[v1.ResourceStorage] = resource.MustParse("8Gi")
		}
	}
}

// setDefaultsFromResourceList fills unspecified fields with a value from given source spec.
func setDefaultsFromResourceList(s *v1.ResourceList, source v1.ResourceList) {
	for k, v := range source {
		if *s == nil {
			*s = make(v1.ResourceList)
		}
		if _, found := (*s)[k]; !found {
			(*s)[k] = v
		}
	}
}

// SetDefaultsFrom fills unspecified fields with a value from given source spec.
func (s *ServerGroupSpec) SetDefaultsFrom(source ServerGroupSpec) {
	if s.Count == nil {
		s.Count = util.NewIntOrNil(source.Count)
	}
	if s.MinCount == nil {
		s.MinCount = util.NewIntOrNil(source.MinCount)
	}
	if s.MaxCount == nil {
		s.MaxCount = util.NewIntOrNil(source.MaxCount)
	}
	if s.Args == nil {
		s.Args = source.Args
	}
	if s.StorageClassName == nil {
		s.StorageClassName = util.NewStringOrNil(source.StorageClassName)
	}
	if s.Tolerations == nil {
		s.Tolerations = source.Tolerations
	}
	if s.ServiceAccountName == nil {
		s.ServiceAccountName = util.NewStringOrNil(source.ServiceAccountName)
	}
	if s.NodeSelector == nil {
		s.NodeSelector = source.NodeSelector
	}
	setDefaultsFromResourceList(&s.Resources.Limits, source.Resources.Limits)
	setDefaultsFromResourceList(&s.Resources.Requests, source.Resources.Requests)
}

// ResetImmutableFields replaces all immutable fields in the given target with values from the source spec.
// It returns a list of fields that have been reset.
func (s ServerGroupSpec) ResetImmutableFields(group ServerGroup, fieldPrefix string, target *ServerGroupSpec) []string {
	var resetFields []string
	if group == ServerGroupAgents {
		if s.GetCount() != target.GetCount() {
			target.Count = util.NewIntOrNil(s.Count)
			resetFields = append(resetFields, fieldPrefix+".count")
		}
	}
	return resetFields
}
