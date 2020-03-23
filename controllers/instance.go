/**
 * 功能描述: 对application中的module进行调谐
 * @Date: 2019-11-14
 * @author: lixiaoming
 */
package controllers

import (
	"context"
	"fmt"
	appv1 "github.com/xm5646/paas-crd-application/api/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ApplicationReconciler) reconcileInstance(app *appv1.Application) error {
	newDeploys := make(map[string]*v1.Deployment)
	for i := range app.Spec.Modules {
		module := &app.Spec.Modules[i]

		deploy, err := makeModule2Deployment(module, app)
		if err != nil {
			log.Error(err, "failed to make module to deployment.", "moduleName", module.Name)
			return err
		}
		if err := controllerutil.SetControllerReference(app, deploy, r.Scheme); err != nil {
			log.Error(err, "failed to set Owner reference for module", "moduleName", module.Name)
			return nil
		}

		newDeploys[deploy.Name] = deploy

		found := &v1.Deployment{}
		err = r.Get(context.TODO(), types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
		// deployment is not found
		if err != nil && apierrs.IsNotFound(err) {
			log.Info("the spec module is not found and create new deployment.", "namespace", app.Namespace, "name", deploy.Name)
			if err = r.Create(context.TODO(), deploy); err != nil {
				log.Error(err, "failed to create new deployment")
				return err
			}
		} else if err != nil {
			// query failed
			log.Error(err, "failed to get deployment.", "namespace", app.Namespace, "name", deploy.Name)
			return err
		} else if !reflect.DeepEqual(deploy.Spec, found.Spec) {
			// 如果版本有更新,则进行update
			// 如果replica数量变化,以集群内状态为准,并反向更新到App.Spec,防止影响到hpa弹性伸缩
			if *deploy.Spec.Replicas != *found.Spec.Replicas {
				log.Info("the replicas was changed, will apply the deployment replicas from cluster", "apply", found.Spec.Replicas, "origin", deploy.Spec.Replicas)
				app.Spec.Modules[i].Template.Replicas = found.Spec.Replicas
				err := r.Update(context.Background(), app)
				if err != nil {
					log.Error(err, "failed to update app module replicas.", "app", app.Name, "module", deploy.Name)
					return err
				}
				r.Recorder.Event(app, "Normal", "SuccessfulUpdated", fmt.Sprintf("Updated module %s replica to %d in %s/%s", found.Name, found.Spec.Replicas, app.Namespace, app.Spec.DisplayName))
				log.Info("Successfully update application module replicas.")
				return nil
			}
			found.Spec = deploy.Spec
			// 清空资源版本, 防止与冲突
			found.ResourceVersion = ""
			err = r.Update(context.TODO(), found)
			if err != nil {
				log.Error(err, "failed to update deployment.", "namespace", app.Namespace, "name", found.Name)
				return err
			}
			r.Recorder.Event(app, "Normal", "SuccessfulUpdated", fmt.Sprintf("Updated module %s in %s/%s", found.Name, app.Namespace, app.Spec.DisplayName))
			log.Info("found deployment has changed and updating by spec module.", "namespace", deploy.Namespace, "name", deploy.Name)
		}

	}

	// 判断是否主动删除module
	return r.cleanUpDeployment(app, newDeploys)
}

func (r *ApplicationReconciler) cleanUpDeployment(app *appv1.Application, newDeployList map[string]*v1.Deployment) error {
	ctx := context.Background()

	deploymentList := &v1.DeploymentList{}
	labels := make(map[string]string)
	labels[APPNameLabel] = app.Name

	if err := r.List(ctx, deploymentList, client.InNamespace(app.Namespace), client.MatchingLabels{APPNameLabel: app.Name}); err != nil {
		log.Error(err, "failed to list deployment by namespace and label.", "namespace", app.Namespace, "label", APPNameLabel)
		return err
	}

	for _, oldDeploy := range deploymentList.Items {
		// 判断属于当前应用的deploy是否还在app.spec内指定,如果未指定,则需要清理该deployment及其相关的资源配置
		if _, isExist := newDeployList[oldDeploy.Name]; isExist == false {
			log.Info("Find an isolated deployment. deleting it.", "namespace", app.Namespace, "deploymentName", oldDeploy.Name)
			r.Recorder.Event(app, "Normal", "Deleting", fmt.Sprintf("Deleting module %s  in %s/%s", oldDeploy.Name, app.Namespace, app.Spec.DisplayName))

			// 如果存在ingress config map ,进行删除
			err := r.cleanUpProxy(types.NamespacedName{Name: oldDeploy.Name, Namespace: app.Namespace})
			if err != nil {
				log.Error(err, "failed to delete ingress configmap.", "namespace", app.Namespace, "deploymentName", oldDeploy.Name)
				return err
			}
			r.Recorder.Event(app, "Normal", "SuccessfulDelete", fmt.Sprintf("Deleted proxy config for moudle %s  in %s/%s", oldDeploy.Name, app.Namespace, app.Spec.DisplayName))

			// 如果存在svc, 则删除对应的svc
			svc := &corev1.Service{}
			err = r.Get(context.TODO(), types.NamespacedName{Namespace: oldDeploy.Namespace, Name: oldDeploy.Name}, svc)
			if err == nil {
				// 存在svc 进行删除
				err = r.Delete(context.TODO(), svc)
				if err != nil {
					log.Error(err, "failed to delete the not defined svc.", "namespace", app.Namespace, "deploymentName", oldDeploy.Name)
					return err
				}
			}
			r.Recorder.Event(app, "Normal", "SuccessfulDelete", fmt.Sprintf("Deleted svc for moudle %s  in %s/%s", oldDeploy.Name, app.Namespace, app.Spec.DisplayName))

			// 孤立的deployment, 进行删除
			err = r.Delete(context.TODO(), &oldDeploy)
			if err != nil {
				log.Error(err, "failed to delete the not defined deployment.", "namespace", app.Namespace, "deploymentName", oldDeploy.Name)
				return err
			}
			r.Recorder.Event(app, "Normal", "SuccessfulDelete", fmt.Sprintf("Deleted moudle %s  in %s/%s", oldDeploy.Name, app.Namespace, app.Spec.DisplayName))

		}
	}

	return nil
}

func makeModule2Deployment(module *appv1.Module, app *appv1.Application) (*v1.Deployment, error) {
	labels := app.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[APPNameLabel] = app.Name
	labels[ModuleNameLabel] = module.Name
	labels[DeploymentType] = "crd"

	deploySpec := module.Template
	if deploySpec.Template.Labels == nil {
		deploySpec.Template.Labels = make(map[string]string)
	}
	deploySpec.Template.Labels[APPNameLabel] = app.Spec.DisplayName
	deploySpec.Template.Labels[ModuleNameLabel] = module.Name
	deploySpec.Template.Labels[PodType] = "crd"

	// 判断是否需要拉取软件包
	deploy := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      module.Name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: deploySpec,
	}

	return deploy, nil
}
