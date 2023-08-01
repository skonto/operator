package knativeserving

import (
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
)

func CompactionTransformers(manifest *mf.Manifest) []mf.Transformer {
	return []mf.Transformer{func(u *unstructured.Unstructured) error {

		if u.GetKind() == "Service" && u.GetName() == "controller" {
			var obj metav1.Object
			controllerSrv := &corev1.Service{}
			if err := scheme.Scheme.Convert(u, controllerSrv, nil); err != nil {
				return err
			}
			obj = controllerSrv

			srvPorts := []corev1.ServicePort{
				{
					Name:       "http-metrics-controller",
					Port:       9091,
					TargetPort: intstr.IntOrString{IntVal: 9091},
				},
				{
					Name:       "http-profiling-controller",
					Port:       8009,
					TargetPort: intstr.IntOrString{IntVal: 8009},
				},
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			}
			controllerSrv.Spec.Ports = append(controllerSrv.Spec.Ports, srvPorts...)
			if err := scheme.Scheme.Convert(obj, u, nil); err != nil {
				return err
			}
		}

		if u.GetKind() == "Deployment" && u.GetName() == "controller" {
			var obj metav1.Object
			controller := &appsv1.Deployment{}
			autoscaler := &appsv1.Deployment{}
			if err := scheme.Scheme.Convert(u, controller, nil); err != nil {
				return err
			}
			obj = controller
			for _, r := range manifest.Resources() {
				if r.GetName() == "autoscaler" && r.GetKind() == "Deployment" {
					if err := scheme.Scheme.Convert(&r, autoscaler, nil); err != nil {
						return err
					}
					// Due to the extension transformations we could have more vendor specific containers here.
					for _, c := range autoscaler.Spec.Template.Spec.Containers {
						if c.Name == "autoscaler" {
							controller.Spec.Template.Spec.Containers = append(controller.Spec.Template.Spec.Containers, c)
						}
					}
					controller.Spec.Strategy = autoscaler.Spec.Strategy
				}
			}

			extraEnvs := []corev1.EnvVar{
				{
					Name:  "K_HEALTH_CHECK_PORT",
					Value: "8085",
				},
				{
					Name:  "PROFILING_PORT",
					Value: "8009",
				},
				{
					Name:  "METRICS_PROMETHEUS_PORT",
					Value: "9091",
				},
			}

			cPorts := []corev1.ContainerPort{
				{
					Name:          "metrics",
					ContainerPort: 9091,
				},
				{
					Name:          "profiling",
					ContainerPort: 8009,
				},
				{
					Name:          "probes",
					ContainerPort: 8085,
				},
			}
			for i, c := range controller.Spec.Template.Spec.Containers {
				if c.Name == "controller" {
					c.Env = append(c.Env, extraEnvs...)
					c.Ports = cPorts
					controller.Spec.Template.Spec.Containers[i] = c
				}
			}

			if err := scheme.Scheme.Convert(obj, u, nil); err != nil {
				return err
			}
		}
		return nil
	}}
}
