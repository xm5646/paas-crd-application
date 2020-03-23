/**
 * 功能描述: 对application中module对应的svc进行调谐
 * @Date: 2019-11-14
 * @author: lixiaoming
 */
package controllers

import (
	appv1 "192.168.31.131/paas-crd/application/api/v1"
	"context"
	"fmt"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"strings"
)

// 对module对应的svc进行调谐
func (r *ApplicationReconciler) reconcileSvc(app *appv1.Application) error {
	for i := range app.Spec.Modules {
		module := &app.Spec.Modules[i]

		// 根据模块名称查找集群中对应的deployment
		deploy := &v1.Deployment{}
		err := r.Get(context.TODO(), types.NamespacedName{Name: module.Name, Namespace: app.Namespace}, deploy)
		if err != nil && strings.Contains(err.Error(), "not found") {
			log.Info("the delployment was not created. continue...")
			continue
		} else if err != nil {
			log.Error(err, "failed to get deploy for svc reconcile.", "namespace", app.Namespace, "name", module.Name)
			return err
		}

		// 根据deployment制作一个svc
		specSvc, err := makeSvcFromDeploy(deploy)
		if err != nil {
			log.Error(err, "failed make svc from deploy.", "namespace", deploy.Namespace, "name", deploy.Name)
			return err
		}

		// 对于没有端口暴露的svc进行删除
		foundSvc := &corev1.Service{}
		if len(specSvc.Spec.Ports) <= 0 {
			log.Info("the svc is no ports", "namespace", deploy.Namespace, "name", deploy.Name)
			err = r.Get(context.TODO(), types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, foundSvc)
			if err == nil {
				err = r.Delete(context.TODO(), foundSvc)
				if err != nil {
					log.Error(err, "failed to delete the no use svc.", "namespace", deploy.Namespace, "name", deploy.Name)
					return err
				}
				log.Info("delete the no use svc.", "namespace", deploy.Namespace, "name", deploy.Name)
			}
			return nil
		}

		// 根据namespaceName获取svc, 如果不存在则创建,如果和预定义不一致,则更新
		err = r.Get(context.TODO(), types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, foundSvc)
		if err != nil && apierrs.IsNotFound(err) {
			if specSvc != nil {
				log.Info("the svc is not found and create new one.", "namespace", deploy.Namespace, "name", deploy.Name)
				if err := r.Create(context.TODO(), specSvc); err != nil {
					log.Error(err, "failed to create svc.", "namespace", deploy.Namespace, "name", deploy.Name)
					return err
				}
				r.Recorder.Event(specSvc, "Normal", "Created", fmt.Sprintf("Create svc for moudle %s  in %s/%s", deploy.Name, app.Namespace, app.Spec.DisplayName))

			}
		} else if err != nil {
			log.Error(err, "failed to get svc", "namespace", deploy.Namespace, "name", deploy.Name)
			return err
		} else if !reflect.DeepEqual(foundSvc.Spec, specSvc.Spec) {
			// 如果不一致,更新svc, 保留原svc ClusterIP
			clusterIP := foundSvc.Spec.ClusterIP
			foundSvc.Spec = specSvc.Spec
			foundSvc.Spec.ClusterIP = clusterIP
			err = r.Update(context.TODO(), foundSvc)
			if err != nil {
				log.Error(err, "failed to update svc", "namespace", deploy.Namespace, "name", deploy.Name)
				return err
			}
			r.Recorder.Event(specSvc, "Normal", "SuccessfulUpdate", fmt.Sprintf("SuccessfulUpdate svc for moudle %s  in %s/%s", deploy.Name, app.Namespace, app.Spec.DisplayName))

		}

	}

	return nil
}

func makeSvcFromDeploy(deploy *v1.Deployment) (*corev1.Service, error) {
	svc := &corev1.Service{}
	svc.ObjectMeta.Labels = deploy.Labels
	svc.ObjectMeta.Name = deploy.Name
	svc.ObjectMeta.Namespace = deploy.Namespace
	svcPorts := make([]corev1.ServicePort, 0, 1)
	startNum := 0
	for _, container := range deploy.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			svcPort := corev1.ServicePort{
				Name:     deploy.Name + "-" + fmt.Sprintf("%d", startNum),
				Port:     port.ContainerPort,
				Protocol: port.Protocol,
				TargetPort: intstr.IntOrString{
					Type:   0,
					IntVal: port.ContainerPort,
					StrVal: fmt.Sprintf("%d", port.ContainerPort),
				},
			}
			startNum += 1
			svcPorts = append(svcPorts, svcPort)
		}
	}
	svc.Spec.Ports = svcPorts
	label := make(map[string]string)
	label["name"] = deploy.Name
	svc.Spec.Selector = label
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	return svc, nil
}
