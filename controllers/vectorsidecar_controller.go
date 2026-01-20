/*
Copyright 2026.

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

package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	observabilityv1alpha1 "github.com/amitde789696/vector-sidecar-operator/api/v1alpha1"
)

const (
	// AnnotationInjected marks a deployment as having the Vector sidecar injected
	AnnotationInjected = "vectorsidecar.observability.amitde789696.io/injected"
	// AnnotationInjectedHash stores the hash of the injected configuration
	AnnotationInjectedHash = "vectorsidecar.observability.amitde789696.io/injected-hash"
	// AnnotationVectorSidecarName stores the name of the VectorSidecar CR
	AnnotationVectorSidecarName = "vectorsidecar.observability.amitde789696.io/sidecar-name"
	// AnnotationConfigMapVersion stores the resourceVersion of the ConfigMap
	AnnotationConfigMapVersion = "vectorsidecar.observability.amitde789696.io/configmap-version"

	// FinalizerName is the finalizer added to VectorSidecar resources
	FinalizerName = "vectorsidecar.observability.amitde789696.io/finalizer"

	// Vector config volume name
	VectorConfigVolumeName = "vector-config"
)

// VectorSidecarReconciler reconciles a VectorSidecar object
type VectorSidecarReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=observability.amitde789696.io,resources=vectorsidecars,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=observability.amitde789696.io,resources=vectorsidecars/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=observability.amitde789696.io,resources=vectorsidecars/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for VectorSidecar resources
// It watches VectorSidecar CRs and Deployments, injecting Vector sidecar containers
// into matching Deployments based on label selectors.
func (r *VectorSidecarReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VectorSidecar", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VectorSidecar instance
	vectorSidecar := &observabilityv1alpha1.VectorSidecar{}
	if err := r.Get(ctx, req.NamespacedName, vectorSidecar); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("VectorSidecar resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get VectorSidecar")
		return ctrl.Result{}, err
	}

	// Handle deletion with finalizer
	if !vectorSidecar.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, vectorSidecar)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(vectorSidecar, FinalizerName) {
		controllerutil.AddFinalizer(vectorSidecar, FinalizerName)
		if err := r.Update(ctx, vectorSidecar); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate the VectorSidecar configuration
	if err := r.validateConfig(ctx, vectorSidecar); err != nil {
		logger.Error(err, "Invalid VectorSidecar configuration")
		r.updateStatusCondition(ctx, vectorSidecar, observabilityv1alpha1.ConditionTypeConfigValid,
			metav1.ConditionFalse, "ValidationFailed", err.Error())
		r.Recorder.Event(vectorSidecar, corev1.EventTypeWarning, "ValidationFailed", err.Error())

		// Persist the status change
		if statusErr := r.Status().Update(ctx, vectorSidecar); statusErr != nil {
			logger.Error(statusErr, "Failed to update status after validation failure")
			return ctrl.Result{}, statusErr
		}

		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	r.updateStatusCondition(ctx, vectorSidecar, observabilityv1alpha1.ConditionTypeConfigValid,
		metav1.ConditionTrue, "ValidationSucceeded", "Configuration is valid")

	// Handle injection based on enabled flag
	if !vectorSidecar.Spec.Enabled {
		logger.Info("VectorSidecar is disabled, removing sidecars from deployments")
		return r.handleDisabledSidecar(ctx, vectorSidecar)
	}

	// Get matching deployments
	matchedDeployments, err := r.getMatchingDeployments(ctx, vectorSidecar)
	if err != nil {
		logger.Error(err, "Failed to get matching deployments")
		r.updateStatusCondition(ctx, vectorSidecar, observabilityv1alpha1.ConditionTypeReady,
			metav1.ConditionFalse, "DeploymentListFailed", err.Error())
		return ctrl.Result{}, err
	}

	logger.Info("Found matching deployments", "count", len(matchedDeployments))

	// Inject sidecar into matching deployments
	injectedCount := 0
	var injectionErrors []string

	for _, deployment := range matchedDeployments {
		if err := r.injectSidecar(ctx, vectorSidecar, &deployment); err != nil {
			logger.Error(err, "Failed to inject sidecar", "deployment", deployment.Name)
			injectionErrors = append(injectionErrors, fmt.Sprintf("%s: %v", deployment.Name, err))
			r.Recorder.Event(vectorSidecar, corev1.EventTypeWarning, "InjectionFailed",
				fmt.Sprintf("Failed to inject into %s: %v", deployment.Name, err))
		} else {
			injectedCount++
			r.Recorder.Event(vectorSidecar, corev1.EventTypeNormal, "InjectionSucceeded",
				fmt.Sprintf("Successfully injected sidecar into %s", deployment.Name))
		}
	}

	// Update status
	vectorSidecar.Status.MatchedDeployments = int32(len(matchedDeployments))
	vectorSidecar.Status.InjectedDeployments = int32(injectedCount)
	vectorSidecar.Status.LastUpdateTime = metav1.Now()
	vectorSidecar.Status.ObservedGeneration = vectorSidecar.Generation

	if len(injectionErrors) > 0 {
		errorMsg := fmt.Sprintf("Injected %d/%d deployments. Errors: %v", injectedCount, len(matchedDeployments), injectionErrors)
		r.updateStatusCondition(ctx, vectorSidecar, observabilityv1alpha1.ConditionTypeReady,
			metav1.ConditionFalse, "InjectionPartiallyFailed", errorMsg)
	} else if injectedCount > 0 {
		r.updateStatusCondition(ctx, vectorSidecar, observabilityv1alpha1.ConditionTypeReady,
			metav1.ConditionTrue, "InjectionSucceeded", fmt.Sprintf("Injected %d deployments", injectedCount))
	} else {
		r.updateStatusCondition(ctx, vectorSidecar, observabilityv1alpha1.ConditionTypeReady,
			metav1.ConditionTrue, "NoMatchingDeployments", "No deployments match the selector")
	}

	if err := r.Status().Update(ctx, vectorSidecar); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciliation complete", "matched", len(matchedDeployments), "injected", injectedCount)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// handleDeletion removes sidecars from all deployments when VectorSidecar is deleted
func (r *VectorSidecarReconciler) handleDeletion(ctx context.Context, vectorSidecar *observabilityv1alpha1.VectorSidecar) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling VectorSidecar deletion")

	if !controllerutil.ContainsFinalizer(vectorSidecar, FinalizerName) {
		return ctrl.Result{}, nil
	}

	// Get all deployments with our annotation
	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(vectorSidecar.Namespace)); err != nil {
		logger.Error(err, "Failed to list deployments for cleanup")
		return ctrl.Result{}, err
	}

	// Remove sidecars from deployments that reference this VectorSidecar
	for _, deployment := range deployments.Items {
		if deployment.Annotations[AnnotationVectorSidecarName] == vectorSidecar.Name {
			if err := r.removeSidecar(ctx, &deployment); err != nil {
				logger.Error(err, "Failed to remove sidecar during deletion", "deployment", deployment.Name)
				return ctrl.Result{}, err
			}
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(vectorSidecar, FinalizerName)
	if err := r.Update(ctx, vectorSidecar); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully cleaned up VectorSidecar")
	return ctrl.Result{}, nil
}

// handleDisabledSidecar removes sidecars when enabled=false
func (r *VectorSidecarReconciler) handleDisabledSidecar(ctx context.Context, vectorSidecar *observabilityv1alpha1.VectorSidecar) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(vectorSidecar.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	removedCount := 0
	for _, deployment := range deployments.Items {
		if deployment.Annotations[AnnotationVectorSidecarName] == vectorSidecar.Name {
			if err := r.removeSidecar(ctx, &deployment); err != nil {
				logger.Error(err, "Failed to remove sidecar", "deployment", deployment.Name)
			} else {
				removedCount++
			}
		}
	}

	vectorSidecar.Status.InjectedDeployments = 0
	vectorSidecar.Status.LastUpdateTime = metav1.Now()
	r.updateStatusCondition(ctx, vectorSidecar, observabilityv1alpha1.ConditionTypeReady,
		metav1.ConditionTrue, "SidecarDisabled", fmt.Sprintf("Removed sidecars from %d deployments", removedCount))

	if err := r.Status().Update(ctx, vectorSidecar); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// validateConfig validates the VectorSidecar configuration
func (r *VectorSidecarReconciler) validateConfig(ctx context.Context, vectorSidecar *observabilityv1alpha1.VectorSidecar) error {
	// Validate that at least one config source is specified
	if vectorSidecar.Spec.Sidecar.Config.ConfigMapRef == nil && vectorSidecar.Spec.Sidecar.Config.Inline == "" {
		return fmt.Errorf("either configMapRef or inline configuration must be specified")
	}

	// If ConfigMapRef is specified, verify the ConfigMap exists
	if vectorSidecar.Spec.Sidecar.Config.ConfigMapRef != nil {
		cm := &corev1.ConfigMap{}
		cmName := types.NamespacedName{
			Name:      vectorSidecar.Spec.Sidecar.Config.ConfigMapRef.Name,
			Namespace: vectorSidecar.Namespace,
		}
		if err := r.Get(ctx, cmName, cm); err != nil {
			return fmt.Errorf("configMap %s not found: %w", cmName.Name, err)
		}

		key := vectorSidecar.Spec.Sidecar.Config.ConfigMapRef.Key
		if key == "" {
			key = "vector.yaml"
		}
		if _, ok := cm.Data[key]; !ok {
			return fmt.Errorf("configMap %s does not contain key %s", cmName.Name, key)
		}
	}

	return nil
}

// getMatchingDeployments returns deployments matching the selector
func (r *VectorSidecarReconciler) getMatchingDeployments(ctx context.Context, vectorSidecar *observabilityv1alpha1.VectorSidecar) ([]appsv1.Deployment, error) {
	deploymentList := &appsv1.DeploymentList{}

	// List deployments in the same namespace
	listOpts := []client.ListOption{
		client.InNamespace(vectorSidecar.Namespace),
	}

	if err := r.List(ctx, deploymentList, listOpts...); err != nil {
		return nil, err
	}

	// Filter deployments by label selector
	selector, err := metav1.LabelSelectorAsSelector(&vectorSidecar.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("invalid selector: %w", err)
	}

	var matched []appsv1.Deployment
	for _, deployment := range deploymentList.Items {
		if selector.Matches(labels.Set(deployment.Labels)) {
			matched = append(matched, deployment)
		}
	}

	return matched, nil
}

// injectSidecar injects the Vector sidecar into a deployment
func (r *VectorSidecarReconciler) injectSidecar(ctx context.Context, vectorSidecar *observabilityv1alpha1.VectorSidecar, deployment *appsv1.Deployment) error {
	logger := log.FromContext(ctx)

	// Calculate the current injection hash
	currentHash, err := r.calculateInjectionHash(vectorSidecar)
	if err != nil {
		return fmt.Errorf("failed to calculate injection hash: %w", err)
	}

	// Check if already injected with the same configuration
	if deployment.Annotations != nil {
		if existingHash, ok := deployment.Annotations[AnnotationInjectedHash]; ok {
			if existingHash == currentHash {
				logger.Info("Deployment already has matching sidecar, skipping", "deployment", deployment.Name)
				return nil
			}
		}
	}

	// Create a copy of the deployment for modification
	deploymentCopy := deployment.DeepCopy()

	// Ensure annotations map exists
	if deploymentCopy.Annotations == nil {
		deploymentCopy.Annotations = make(map[string]string)
	}
	if deploymentCopy.Spec.Template.Annotations == nil {
		deploymentCopy.Spec.Template.Annotations = make(map[string]string)
	}

	// Remove existing Vector container if present
	containers := []corev1.Container{}
	sidecarName := vectorSidecar.Spec.Sidecar.Name
	if sidecarName == "" {
		sidecarName = "vector"
	}

	for _, container := range deploymentCopy.Spec.Template.Spec.Containers {
		if container.Name != sidecarName {
			containers = append(containers, container)
		}
	}

	// Build the Vector sidecar container
	vectorContainer := r.buildVectorContainer(vectorSidecar)
	containers = append(containers, vectorContainer)
	deploymentCopy.Spec.Template.Spec.Containers = containers

	// Handle volumes
	if err := r.injectVolumes(vectorSidecar, deploymentCopy); err != nil {
		return fmt.Errorf("failed to inject volumes: %w", err)
	}

	// Handle init containers
	if len(vectorSidecar.Spec.InitContainers) > 0 {
		deploymentCopy.Spec.Template.Spec.InitContainers = append(
			deploymentCopy.Spec.Template.Spec.InitContainers,
			vectorSidecar.Spec.InitContainers...,
		)
	}

	// Update annotations
	deploymentCopy.Annotations[AnnotationInjected] = "true"
	deploymentCopy.Annotations[AnnotationInjectedHash] = currentHash
	deploymentCopy.Annotations[AnnotationVectorSidecarName] = vectorSidecar.Name

	// Store ConfigMap version if using ConfigMapRef
	if vectorSidecar.Spec.Sidecar.Config.ConfigMapRef != nil {
		cm := &corev1.ConfigMap{}
		cmName := types.NamespacedName{
			Name:      vectorSidecar.Spec.Sidecar.Config.ConfigMapRef.Name,
			Namespace: vectorSidecar.Namespace,
		}
		if err := r.Get(ctx, cmName, cm); err == nil {
			deploymentCopy.Annotations[AnnotationConfigMapVersion] = cm.ResourceVersion
		}
	}

	deploymentCopy.Spec.Template.Annotations[AnnotationInjectedHash] = currentHash

	// Update the deployment
	if err := r.Update(ctx, deploymentCopy); err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	logger.Info("Successfully injected sidecar", "deployment", deployment.Name, "hash", currentHash)
	return nil
}

// removeSidecar removes the Vector sidecar from a deployment
func (r *VectorSidecarReconciler) removeSidecar(ctx context.Context, deployment *appsv1.Deployment) error {
	logger := log.FromContext(ctx)

	deploymentCopy := deployment.DeepCopy()

	// Remove Vector container
	containers := []corev1.Container{}
	for _, container := range deploymentCopy.Spec.Template.Spec.Containers {
		if container.Name != "vector" {
			containers = append(containers, container)
		}
	}
	deploymentCopy.Spec.Template.Spec.Containers = containers

	// Remove Vector config volume
	volumes := []corev1.Volume{}
	for _, volume := range deploymentCopy.Spec.Template.Spec.Volumes {
		if volume.Name != VectorConfigVolumeName {
			volumes = append(volumes, volume)
		}
	}
	deploymentCopy.Spec.Template.Spec.Volumes = volumes

	// Remove annotations
	delete(deploymentCopy.Annotations, AnnotationInjected)
	delete(deploymentCopy.Annotations, AnnotationInjectedHash)
	delete(deploymentCopy.Annotations, AnnotationVectorSidecarName)
	delete(deploymentCopy.Annotations, AnnotationConfigMapVersion)
	delete(deploymentCopy.Spec.Template.Annotations, AnnotationInjectedHash)

	if err := r.Update(ctx, deploymentCopy); err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	logger.Info("Successfully removed sidecar", "deployment", deployment.Name)
	return nil
}

// buildVectorContainer builds the Vector sidecar container spec
func (r *VectorSidecarReconciler) buildVectorContainer(vectorSidecar *observabilityv1alpha1.VectorSidecar) corev1.Container {
	sidecarSpec := vectorSidecar.Spec.Sidecar

	containerName := sidecarSpec.Name
	if containerName == "" {
		containerName = "vector"
	}

	container := corev1.Container{
		Name:            containerName,
		Image:           sidecarSpec.Image,
		ImagePullPolicy: sidecarSpec.ImagePullPolicy,
		Resources:       sidecarSpec.Resources,
		Env:             sidecarSpec.Env,
	}

	// Set default ImagePullPolicy if not specified
	if container.ImagePullPolicy == "" {
		container.ImagePullPolicy = corev1.PullIfNotPresent
	}

	// Build args
	args := []string{}
	if len(sidecarSpec.Args) > 0 {
		args = append(args, sidecarSpec.Args...)
	}

	// Add config file argument
	if sidecarSpec.Config.ConfigMapRef != nil {
		args = append(args, "--config", "/etc/vector/vector.yaml")
	} else if sidecarSpec.Config.Inline != "" {
		args = append(args, "--config", "/etc/vector/vector.yaml")
	}

	container.Args = args

	// Add volume mounts
	volumeMounts := []corev1.VolumeMount{}

	// Add config volume mount
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      VectorConfigVolumeName,
		MountPath: "/etc/vector",
		ReadOnly:  true,
	})

	// Add custom volume mounts
	volumeMounts = append(volumeMounts, sidecarSpec.VolumeMounts...)
	container.VolumeMounts = volumeMounts

	return container
}

// injectVolumes adds necessary volumes to the deployment
func (r *VectorSidecarReconciler) injectVolumes(vectorSidecar *observabilityv1alpha1.VectorSidecar, deployment *appsv1.Deployment) error {
	volumes := deployment.Spec.Template.Spec.Volumes

	// Remove existing vector-config volume if present
	filteredVolumes := []corev1.Volume{}
	for _, vol := range volumes {
		if vol.Name != VectorConfigVolumeName {
			filteredVolumes = append(filteredVolumes, vol)
		}
	}
	volumes = filteredVolumes

	// Add Vector config volume
	configVolume := corev1.Volume{
		Name: VectorConfigVolumeName,
	}

	if vectorSidecar.Spec.Sidecar.Config.ConfigMapRef != nil {
		key := vectorSidecar.Spec.Sidecar.Config.ConfigMapRef.Key
		if key == "" {
			key = "vector.yaml"
		}
		configVolume.VolumeSource = corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: vectorSidecar.Spec.Sidecar.Config.ConfigMapRef.Name,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  key,
						Path: "vector.yaml",
					},
				},
			},
		}
	} else if vectorSidecar.Spec.Sidecar.Config.Inline != "" {
		// For inline config, create a ConfigMap
		cmName := fmt.Sprintf("%s-inline-config", vectorSidecar.Name)
		configVolume.VolumeSource = corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmName,
				},
			},
		}
	}

	volumes = append(volumes, configVolume)

	// Add custom volumes from spec
	for _, vol := range vectorSidecar.Spec.Volumes {
		// Check if volume already exists
		exists := false
		for _, existingVol := range volumes {
			if existingVol.Name == vol.Name {
				exists = true
				break
			}
		}
		if !exists {
			volumes = append(volumes, vol)
		}
	}

	deployment.Spec.Template.Spec.Volumes = volumes
	return nil
}

// calculateInjectionHash calculates a hash of the injection configuration
func (r *VectorSidecarReconciler) calculateInjectionHash(vectorSidecar *observabilityv1alpha1.VectorSidecar) (string, error) {
	// Create a struct containing all relevant fields for hashing
	hashData := struct {
		Image        string
		Config       observabilityv1alpha1.VectorConfig
		VolumeMounts []corev1.VolumeMount
		Resources    corev1.ResourceRequirements
		Env          []corev1.EnvVar
		Args         []string
		Volumes      []corev1.Volume
	}{
		Image:        vectorSidecar.Spec.Sidecar.Image,
		Config:       vectorSidecar.Spec.Sidecar.Config,
		VolumeMounts: vectorSidecar.Spec.Sidecar.VolumeMounts,
		Resources:    vectorSidecar.Spec.Sidecar.Resources,
		Env:          vectorSidecar.Spec.Sidecar.Env,
		Args:         vectorSidecar.Spec.Sidecar.Args,
		Volumes:      vectorSidecar.Spec.Volumes,
	}

	// Marshal to JSON for consistent hashing
	jsonData, err := json.Marshal(hashData)
	if err != nil {
		return "", err
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(jsonData)
	return fmt.Sprintf("%x", hash[:8]), nil
}

// updateStatusCondition updates or adds a condition to the status
func (r *VectorSidecarReconciler) updateStatusCondition(ctx context.Context, vectorSidecar *observabilityv1alpha1.VectorSidecar,
	conditionType string, status metav1.ConditionStatus, reason, message string) {

	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: vectorSidecar.Generation,
	}

	meta.SetStatusCondition(&vectorSidecar.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *VectorSidecarReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.VectorSidecar{}).
		Owns(&appsv1.Deployment{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
