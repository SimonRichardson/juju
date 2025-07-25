// Copyright 2021 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package application

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/juju/errors"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juju/juju/core/arch"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/internal/provider/kubernetes/constants"
)

// ConstraintApplier defines a function type that applies a resource constraint
// (e.g., memory or CPU) with the given value to the specified resource name
// in the provided PodSpec.
type ConstraintApplier func(pod *core.PodSpec, resourceName core.ResourceName, value string) error

// ApplyWorkloadConstraints applies the specified constraints to the pod.
func ApplyWorkloadConstraints(pod *core.PodSpec, appName string, cons constraints.Value, configureConstraint ConstraintApplier) error {
	// TODO(allow resource limits to be applied to each container).
	// For now we only do resource requests, one container is sufficient for
	// scheduling purposes.
	if mem := cons.Mem; mem != nil {
		if err := configureConstraint(pod, core.ResourceMemory, fmt.Sprintf("%dMi", *mem)); err != nil {
			return errors.Annotatef(err, "configuring memory constraint for %s", appName)
		}
	}
	if cpu := cons.CpuPower; cpu != nil {
		if err := configureConstraint(pod, core.ResourceCPU, fmt.Sprintf("%dm", *cpu)); err != nil {
			return errors.Annotatef(err, "configuring cpu constraint for %s", appName)
		}
	}
	nodeSelector := map[string]string(nil)
	if cons.HasArch() {
		cpuArch := *cons.Arch
		cpuArch = arch.NormaliseArch(cpuArch)
		// Convert to Golang arch string
		switch cpuArch {
		case arch.AMD64:
			cpuArch = "amd64"
		case arch.ARM64:
			cpuArch = "arm64"
		case arch.PPC64EL:
			cpuArch = "ppc64le"
		case arch.S390X:
			cpuArch = "s390x"
		default:
			return errors.NotSupportedf("architecture %q", cpuArch)
		}
		nodeSelector = map[string]string{"kubernetes.io/arch": cpuArch}
	}
	if pod.NodeSelector != nil {
		for k, v := range nodeSelector {
			pod.NodeSelector[k] = v
		}
	} else if nodeSelector != nil {
		pod.NodeSelector = nodeSelector
	}

	// Translate tags to pod or node affinity.
	// Tag names are prefixed with "pod.", "anti-pod.", or "node."
	// with the default being "node".
	// The tag 'topology-key', if set, is used for the affinity topology key value.
	if cons.Tags != nil {
		affinityLabels := make(map[string]string)
		for _, labelPair := range *cons.Tags {
			parts := strings.Split(labelPair, "=")
			if len(parts) != 2 {
				return errors.Errorf("invalid affinity constraints: %v", affinityLabels)
			}
			key := strings.Trim(parts[0], " ")
			value := strings.Trim(parts[1], " ")
			affinityLabels[key] = value
		}

		if err := processNodeAffinity(pod, affinityLabels); err != nil {
			return errors.Annotatef(err, "configuring node affinity for %s", appName)
		}
		if err := processPodAffinity(pod, affinityLabels); err != nil {
			return errors.Annotatef(err, "configuring pod affinity for %s", appName)
		}
	}
	if cons.Zones != nil {
		zones := *cons.Zones
		affinity := pod.Affinity
		if affinity == nil {
			affinity = &core.Affinity{
				NodeAffinity: &core.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &core.NodeSelector{
						NodeSelectorTerms: []core.NodeSelectorTerm{{}},
					},
				},
			}
			pod.Affinity = affinity
		}
		selector := &affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
		selector.MatchExpressions = append(selector.MatchExpressions,
			core.NodeSelectorRequirement{
				Key:      "failure-domain.beta.kubernetes.io/zone",
				Operator: core.NodeSelectorOpIn,
				Values:   zones,
			})
	}
	return nil
}

func isMemResourceValueValid(memVal string) bool {
	if !strings.HasSuffix(memVal, "Mi") {
		return false
	}
	val := strings.TrimSuffix(memVal, "Mi")
	num, err := strconv.Atoi(val)
	if err != nil {
		return false
	}
	return num > 0
}

// ApplyCharmConstraints applies the specified charm constraints to the charm container.
func ApplyCharmConstraints(pod *core.PodSpec, appName string,
	charmContainerResourceRequirements CharmContainerResourceRequirements) error {

	if len(pod.Containers) == 0 {
		return nil
	}

	if !isMemResourceValueValid(charmContainerResourceRequirements.MemLimitMi) {
		return errors.NotValidf("charm container mem limit value")
	}

	if !isMemResourceValueValid(charmContainerResourceRequirements.MemRequestMi) {
		return errors.NotValidf("charm container mem request value")
	}

	requestValue := charmContainerResourceRequirements.MemRequestMi
	limitValue := charmContainerResourceRequirements.MemLimitMi

	charmContainerIndex := -1

	for i, container := range pod.Containers {
		if container.Name == constants.ApplicationCharmContainer {
			charmContainerIndex = i
			break
		}
	}

	// If the charm container is not found, we do not apply the constraints.
	if charmContainerIndex == -1 {
		return nil
	}
	var err error

	if pod.Containers[charmContainerIndex].Resources.Requests, err = MergeConstraint(core.ResourceMemory, requestValue, pod.Containers[charmContainerIndex].Resources.Requests); err != nil {
		return errors.Annotatef(err, "merging request constraint %s=%s for charm container", core.ResourceMemory, requestValue)
	}

	if pod.Containers[charmContainerIndex].Resources.Limits, err = MergeConstraint(core.ResourceMemory, limitValue, pod.Containers[charmContainerIndex].Resources.Limits); err != nil {
		return errors.Annotatef(err, "merging limit constraint %s=%s for charm container", core.ResourceMemory, limitValue)
	}

	return nil
}

const (
	podPrefix      = "pod."
	antiPodPrefix  = "anti-pod."
	topologyKeyTag = "topology-key"
	nodePrefix     = "node."
)

func processNodeAffinity(pod *core.PodSpec, affinityLabels map[string]string) error {
	affinityTags := make(map[string]string)
	for key, value := range affinityLabels {
		keyVal := key
		if strings.HasPrefix(key, "^") {
			if len(key) == 1 {
				return errors.Errorf("invalid affinity constraints: %v", affinityLabels)
			}
			key = key[1:]
		}
		if strings.HasPrefix(key, podPrefix) || strings.HasPrefix(key, antiPodPrefix) {
			continue
		}
		key = strings.TrimPrefix(keyVal, nodePrefix)
		affinityTags[key] = value
	}

	updateSelectorTerms := func(nodeSelectorTerm *core.NodeSelectorTerm, tags map[string]string) {
		// Sort for stable ordering.
		var keys []string
		for k := range tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, tag := range keys {
			allValues := strings.Split(tags[tag], "|")
			for i, v := range allValues {
				allValues[i] = strings.Trim(v, " ")
			}
			op := core.NodeSelectorOpIn
			if strings.HasPrefix(tag, "^") {
				tag = tag[1:]
				op = core.NodeSelectorOpNotIn
			}
			nodeSelectorTerm.MatchExpressions = append(nodeSelectorTerm.MatchExpressions, core.NodeSelectorRequirement{
				Key:      tag,
				Operator: op,
				Values:   allValues,
			})
		}
	}
	var nodeSelectorTerm core.NodeSelectorTerm
	updateSelectorTerms(&nodeSelectorTerm, affinityTags)
	if len(nodeSelectorTerm.MatchExpressions) > 0 {
		if pod.Affinity == nil {
			pod.Affinity = &core.Affinity{}
		}
		pod.Affinity.NodeAffinity = &core.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &core.NodeSelector{
				NodeSelectorTerms: []core.NodeSelectorTerm{nodeSelectorTerm},
			},
		}
	}
	return nil
}

func processPodAffinity(pod *core.PodSpec, affinityLabels map[string]string) error {
	affinityTags := make(map[string]string)
	antiAffinityTags := make(map[string]string)
	for key, value := range affinityLabels {
		notVal := false
		if strings.HasPrefix(key, "^") {
			if len(key) == 1 {
				return errors.Errorf("invalid affinity constraints: %v", affinityLabels)
			}
			notVal = true
			key = key[1:]
		}
		if !strings.HasPrefix(key, podPrefix) && !strings.HasPrefix(key, antiPodPrefix) {
			continue
		}
		if strings.HasPrefix(key, podPrefix) {
			key = strings.TrimPrefix(key, podPrefix)
			if notVal {
				key = "^" + key
			}
			affinityTags[key] = value
		}
		if strings.HasPrefix(key, antiPodPrefix) {
			key = strings.TrimPrefix(key, antiPodPrefix)
			if notVal {
				key = "^" + key
			}
			antiAffinityTags[key] = value
		}
	}
	if len(affinityTags) == 0 && len(antiAffinityTags) == 0 {
		return nil
	}

	updateAffinityTerm := func(affinityTerm *core.PodAffinityTerm, tags map[string]string) {
		// Sort for stable ordering.
		var keys []string
		for k := range tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var (
			labelSelector v1.LabelSelector
			topologyKey   string
		)
		for _, tag := range keys {
			if tag == topologyKeyTag {
				topologyKey = tags[tag]
				continue
			}
			allValues := strings.Split(tags[tag], "|")
			for i, v := range allValues {
				allValues[i] = strings.Trim(v, " ")
			}
			op := v1.LabelSelectorOpIn
			if strings.HasPrefix(tag, "^") {
				tag = tag[1:]
				op = v1.LabelSelectorOpNotIn
			}
			labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, v1.LabelSelectorRequirement{
				Key:      tag,
				Operator: op,
				Values:   allValues,
			})
		}
		affinityTerm.LabelSelector = &labelSelector
		if topologyKey != "" {
			affinityTerm.TopologyKey = topologyKey
		}
	}
	var affinityTerm core.PodAffinityTerm
	updateAffinityTerm(&affinityTerm, affinityTags)
	if len(affinityTerm.LabelSelector.MatchExpressions) > 0 || affinityTerm.TopologyKey != "" {
		if pod.Affinity == nil {
			pod.Affinity = &core.Affinity{}
		}
		pod.Affinity.PodAffinity = &core.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []core.PodAffinityTerm{affinityTerm},
		}
	}

	var antiAffinityTerm core.PodAffinityTerm
	updateAffinityTerm(&antiAffinityTerm, antiAffinityTags)
	if len(antiAffinityTerm.LabelSelector.MatchExpressions) > 0 || antiAffinityTerm.TopologyKey != "" {
		if pod.Affinity == nil {
			pod.Affinity = &core.Affinity{}
		}
		pod.Affinity.PodAntiAffinity = &core.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []core.PodAffinityTerm{antiAffinityTerm},
		}
	}
	return nil
}

func configureWorkloadConstraint(pod *core.PodSpec, resourceName core.ResourceName, value string) (err error) {
	if len(pod.Containers) == 0 {
		return nil
	}
	pod.Containers[0].Resources.Requests, err = MergeConstraint(resourceName, value, pod.Containers[0].Resources.Requests)
	if err != nil {
		return errors.Annotatef(err, "merging request constraint %s=%s", resourceName, value)
	}
	for i := range pod.Containers {
		pod.Containers[i].Resources.Limits, err = MergeConstraint(resourceName, value, pod.Containers[i].Resources.Limits)
		if err != nil {
			return errors.Annotatef(err, "merging limit constraint %s=%s", resourceName, value)
		}
	}
	return nil
}

// configureWorkloadConstraintV2 sets the resource limits and requests for all containers,
// excluding the charm container.
func configureWorkloadConstraintV2(pod *core.PodSpec, resourceName core.ResourceName, value string) (err error) {
	if len(pod.Containers) == 0 {
		return nil
	}

	for i, container := range pod.Containers {
		isCharmContainer := container.Name == constants.ApplicationCharmContainer
		if isCharmContainer {
			continue
		}

		pod.Containers[i].Resources.Requests, err = MergeConstraint(resourceName, value, container.Resources.Requests)
		if err != nil {
			return errors.Annotatef(err,
				"merging request constraint %s=%s for container %s", resourceName, value, container.Name)
		}

		pod.Containers[i].Resources.Limits, err = MergeConstraint(resourceName, value, container.Resources.Limits)
		if err != nil {
			return errors.Annotatef(err, "merging limit constraint %s=%s, for container %s", resourceName, value, container.Name)
		}

	}
	return nil
}

// MergeConstraint merges constraint spec.
func MergeConstraint(resourceName core.ResourceName, value string, resourcesList core.ResourceList) (core.ResourceList, error) {
	if resourcesList == nil {
		resourcesList = core.ResourceList{}
	}
	if v, ok := resourcesList[resourceName]; ok {
		return nil, errors.NotValidf("resource list for %q has already been set to %v", resourceName, v)
	}
	parsedValue, err := resource.ParseQuantity(value)
	if err != nil {
		return nil, errors.Annotatef(err, "invalid constraint value %q for %q", value, resourceName)
	}
	resourcesList[resourceName] = parsedValue
	return resourcesList, nil
}
