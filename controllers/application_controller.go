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

package controllers

import (
	appv1 "192.168.31.131/paas-crd/application/api/v1"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

var log = logf.Log.WithName("controller")

var (
	APPNameLabel    = "app.dsgkinfo.com/appName"
	ModuleNameLabel = "app.dsgkinfo.com/moduleName"
	PodType         = "app.dsgkinfo.com/podType"
	DeploymentType  = "app.dsgkinfo.com/deploymentType"
	LocalCache      = make(map[string]string)
)

// +kubebuilder:rbac:groups=app.dsgkinfo.com,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.dsgkinfo.com,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;list;watch;create;update;patch;delete

func (r *ApplicationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("application", req.NamespacedName)
	log.Info("reconciling...")

	var app appv1.Application
	err := r.Get(ctx, req.NamespacedName, &app)
	if err != nil {
		if apierrs.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err, "unable to fetch application")
		return ctrl.Result{}, err
	}
	log.Info("get app successful.", "display name", app.Spec.DisplayName)

	// 判断是否在删除中
	FinalizerName := "finalizers.app.dsgkinfo.com"
	if app.ObjectMeta.DeletionTimestamp.IsZero() {
		// 如果为0, 则表示正常运行
		// 判断是否包含finalizers字段
		if !containsString(app.ObjectMeta.Finalizers, FinalizerName) {
			app.ObjectMeta.Finalizers = append(app.ObjectMeta.Finalizers, FinalizerName)
			err = r.Update(ctx, &app)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// 如果不为0, 则表示正在删除
		if containsString(app.ObjectMeta.Finalizers, FinalizerName) {
			// 进行pre delete
			log.Info("the app will be deleted, update the app status to deleting.")
			r.Recorder.Event(&app, "Normal", "Killing", fmt.Sprintf("Deleting application %s/%s", app.Namespace, app.Spec.DisplayName))
			app.Status.Status = "Deleting"
			err = r.Update(ctx, &app)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info("updated app status successful.", "status", app.Status.Status)
			log.Info("the app will be deleted, deleting dependence resource.")
			err = r.deleteDependenceResource(&app)
			if err != nil {
				log.Error(err, "failed to delete dependence resource.", "namespace", app.Namespace, "applicationName", app.Name)
				return ctrl.Result{}, err
			}
			log.Info("successful delete dependence resource.")
			r.Recorder.Event(&app, "Normal", "SuccessfulDelete", fmt.Sprintf("Deleted application %s/%s", app.Namespace, app.Spec.DisplayName))

			// 删除成功,清空finalizers
			app.ObjectMeta.Finalizers = removeString(app.ObjectMeta.Finalizers, FinalizerName)
			err = r.Update(ctx, &app)
			return ctrl.Result{}, err
		}
	}

	// 对status进行调谐
	log.Info("reconcile status...", "display name", app.Spec.DisplayName)
	if err := r.reconcileStatus(&app); err != nil {
		log.Error(err, "failed to reconcile status.", "namespace", app.Namespace, "applicationName", app.Namespace)
		return ctrl.Result{}, err
	}

	// 进行Module实例调谐
	log.Info("reconcile instance...", "display name", app.Spec.DisplayName)
	if err := r.reconcileInstance(&app); err != nil {
		log.Error(err, "failed to reconcile instance.", "namespace", app.Namespace, "applicationName", app.Name)
		return ctrl.Result{}, err
	}

	// 对svc进行调谐
	log.Info("reconcile svc...", "display name", app.Spec.DisplayName)
	if err := r.reconcileSvc(&app); err != nil {
		log.Error(err, "failed to reconcile svc.", "namespace", app.Namespace, "applicationName", app.Namespace)
		return ctrl.Result{}, err
	}

	// 对proxy进行调谐
	log.Info("reconcile proxy...", "display name", app.Spec.DisplayName)
	if err := r.reconcileProxy(&app); err != nil {
		log.Error(err, "failed to reconcile proxy.", "namespace", app.Namespace, "applicationName", app.Name)
		return ctrl.Result{}, err
	}

	log.Info("reconcile all done.", "display name", app.Spec.DisplayName)
	return ctrl.Result{}, nil
}

var (
	deploymentOwnKey = ".metadata.controller"
	apiGVStr         = appv1.GroupVersion.String()
)

func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 设置查询索引
	if err := mgr.GetFieldIndexer().IndexField(&v1.Deployment{}, deploymentOwnKey, func(object runtime.Object) []string {
		deploy := object.(*v1.Deployment)
		owner := metav1.GetControllerOf(deploy)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "Application" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1.Application{}).
		Owns(&v1.Deployment{}).
		Complete(r)
}
