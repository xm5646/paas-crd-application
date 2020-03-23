/**
 * 功能描述: 对application的状态进行调谐
 * @Date: 2019-11-15
 * @author: lixiaoming
 */
package controllers

import (
	appv1 "192.168.31.131/paas-crd/application/api/v1"
	"context"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"
)

// 对应用的状态进行更新
func (r *ApplicationReconciler) reconcileStatus(app *appv1.Application) error {
	// 根据module获取对应的deployment
	totalNum := int32(0)
	StoppedNum := int32(0)
	RollingUpdateNum := int32(0)
	RunningNum := int32(0)
	StartingNum := int32(0)
	if len(app.Spec.Modules) == 0 {
		app.Status.Status = "Stopped"
		app.Status.RunningModuleNumber = 0
		app.Status.TotalModuleNumber = 0
		err := r.Status().Update(context.TODO(), app)
		if err != nil {
			log.Error(err, "failed to update app.", "namespace", app.Namespace, "applicationName", app.Name)
			return err
		}
		return nil
	}
	for i := range app.Spec.Modules {
		module := app.Spec.Modules[i]
		totalNum += 1
		deploy := &appsv1.Deployment{}
		err := r.Get(context.TODO(), types.NamespacedName{Namespace: app.Namespace, Name: module.Name}, deploy)
		if err != nil && strings.Contains(err.Error(), "not found") {
			log.Info("not to found the module.", "namespace", app.Namespace, "moduleName", module.Name)
			continue
		} else if err != nil {
			log.Error(err, "failed to get deployment from cluster.", "namespace", app.Namespace, "moduleName", module.Name)
			return err
		}
		if *deploy.Spec.Replicas == 0 {
			StoppedNum += 1
		}
		if *deploy.Spec.Replicas > 0 && deploy.Status.AvailableReplicas == 0 {
			StartingNum += 1
		}
		if deploy.Status.UpdatedReplicas < *deploy.Spec.Replicas && deploy.Status.Replicas > *deploy.Spec.Replicas {
			RollingUpdateNum += 1
		}
		if *deploy.Spec.Replicas > 0 && deploy.Status.AvailableReplicas > 0 {
			RunningNum += 1
		}

	}
	if !app.ObjectMeta.DeletionTimestamp.IsZero() {
		app.Status.Status = "Deleting"
	} else if RunningNum > 0 {
		app.Status.Status = "Running"
	} else if RunningNum == 0 && StartingNum > 0 {
		app.Status.Status = "Starting"
	} else if StoppedNum == totalNum {
		app.Status.Status = "Stopped"
	} else {
		app.Status.Status = "Starting"
	}
	app.Status.RunningModuleNumber = RunningNum
	app.Status.TotalModuleNumber = totalNum
	app.Status.StartingModuleNumber = StartingNum
	app.Status.RollingUpdateNumber = RollingUpdateNum
	app.Status.StoppedModuleNumber = StoppedNum
	err := r.Status().Update(context.TODO(), app)
	if err != nil {
		log.Error(err, "failed to update app.", "namespace", app.Namespace, "applicationName", app.Name)
		return err
	}
	return nil
}
