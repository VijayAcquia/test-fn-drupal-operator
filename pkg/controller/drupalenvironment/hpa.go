package drupalenvironment

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const drupalHPAName = "drupal"

func (rh *requestHandler) hpa() *autoscalingv1.HorizontalPodAutoscaler {
	drupalSpec := rh.env.Spec.Drupal
	targetmetric := drupalSpec.TargetCPUUtilizationPercentage
	// FIXME: default values for CRDs
	if targetmetric == nil {
		tmp := int32(50)
		targetmetric = &tmp
	}

	hpa := &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drupalHPAName,
			Namespace: rh.namespace,
			Labels:    rh.env.ChildLabels(),
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
			MinReplicas: &drupalSpec.MinReplicas,
			MaxReplicas: drupalSpec.MaxReplicas,
			ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: "argoproj.io/v1alpha1",
				Kind:       "Rollout",
				Name:       drupalRolloutName,
			},
			TargetCPUUtilizationPercentage: targetmetric,
		},
	}
	return hpa
}

func (rh *requestHandler) reconcileHPA() (bool, error) {
	r := rh.reconciler

	hpa := rh.hpa()

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, hpa, func(existing runtime.Object) error {
		realHPA := existing.(*autoscalingv1.HorizontalPodAutoscaler)
		realHPA.Spec = rh.hpa().Spec
		if realHPA.CreationTimestamp.IsZero() {
			rh.associateResourceWithController(realHPA)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		rh.logger.Info("Successfully reconciled HPA", "operation", op)
		return true, nil
	}
	return false, nil
}
