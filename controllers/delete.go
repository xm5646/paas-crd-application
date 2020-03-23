/**
 * 功能描述: 在app删除时删除相关资源
 * @Date: 2019-11-15
 * @author: lixiaoming
 */
package controllers

import (
	appv1 "192.168.31.131/paas-crd/application/api/v1"
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ApplicationReconciler) deleteDependenceResource(app *appv1.Application) error {
	for i := range app.Spec.Modules {
		module := &app.Spec.Modules[i]
		// 删除module对应的svc
		svc := &corev1.Service{}
		err := r.Get(context.TODO(), types.NamespacedName{Name: module.Name, Namespace: app.Namespace}, svc)
		if err == nil {
			err = r.Delete(context.TODO(), svc)
			if err != nil {
				log.Error(err, "failed to delete the isolated svc.", "namespace", app.Namespace, "name", module.Name)
				return err
			}
			log.Info("deleted the isolated svc.", "namespace", app.Namespace, "name", module.Name)
		}

		// 删除module中定义的proxy规则
		err = r.cleanUpProxy(types.NamespacedName{Namespace: app.Namespace, Name: module.Name})
		if err != nil {
			log.Error(err, "failed to clean ingress config map.", "namespace", app.Namespace, "name", module.Name)
			return err
		}
		log.Info("deleted the ingress L4 rules.")
	}
	return nil
}
