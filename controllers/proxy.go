/**
 * 功能描述: 对application中module的proxy设置进行调谐
 * @Date: 2019-11-14
 * @author: lixiaoming
 */
package controllers

import (
	appv1 "192.168.31.131/paas-crd/application/api/v1"
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"strings"
)

var (
	KubeSystemNamespace = "kube-system"
	IngressTCPConfigMap = "tcp-services"
	IngressUDPConfigMap = "udp-services"
)

func (r *ApplicationReconciler) reconcileProxy(app *appv1.Application) error {
	for i := range app.Spec.Modules {
		module := &app.Spec.Modules[i]

		// 判断是否需要集群外部访问
		if module.AccessMode != "outside" {
			// 不需要外部访问
			module.Proxies = nil
		}

		tcpProxyMap := make(map[string]string)
		udpProxyMap := make(map[string]string)
		// 根据预定义内容生成期望的tcp/udp规则
		if len(module.Proxies) > 0 {
			for _, proxy := range module.Proxies {
				if proxy.Protocol == "tcp" || proxy.Protocol == "TCP" {
					tcpProxyMap[fmt.Sprintf("%d", proxy.TargetPort)] = fmt.Sprintf("%s/%s:%d", app.Namespace, module.Name, proxy.Port)
				} else if proxy.Protocol == "udp" || proxy.Protocol == "UDP" {
					udpProxyMap[fmt.Sprintf("%d", proxy.TargetPort)] = fmt.Sprintf("%s/%s:%d", app.Namespace, module.Name, proxy.Port)
				}
			}
		}

		// get tcp and udp ingress configmap
		tcpConfigMap := &corev1.ConfigMap{}
		err := r.Get(context.TODO(), types.NamespacedName{Namespace: KubeSystemNamespace, Name: IngressTCPConfigMap}, tcpConfigMap)
		if err != nil {
			log.Error(err, "failed to get ingress config map for tcp")
			return err
		}
		if tcpConfigMap.Data == nil {
			tcpConfigMap.Data = make(map[string]string)
		}

		udpConfigMap := &corev1.ConfigMap{}
		err = r.Get(context.TODO(), types.NamespacedName{Namespace: KubeSystemNamespace, Name: IngressUDPConfigMap}, udpConfigMap)
		if err != nil {
			log.Error(err, "failed to get ingress config map for udp")
			return err
		}
		if udpConfigMap.Data == nil {
			udpConfigMap.Data = make(map[string]string)
		}

		// 获取 tcp config map 中本module中的规则
		tcpRules := GetProxyRulesForNameSpaceName(types.NamespacedName{Name: module.Name, Namespace: app.Namespace}, tcpConfigMap)
		// 比对已有规则和期望规则是否一致,不一致清空已有规则,重新添加期望规则
		if !reflect.DeepEqual(tcpRules, tcpProxyMap) {
			// 如果proxy 发生变化,清空原有配置
			log.Info("the proxy has changed, update ingress config map for tcp.")
			for key, _ := range tcpRules {
				delete(tcpConfigMap.Data, key)
			}

			// 检查端口是否被占用
			for key := range tcpProxyMap {
				_, isExist := tcpConfigMap.Data[key]
				if isExist {
					log.Error(nil, "th tcp port is already used.", "protocol", "tcp", "port", key)
					return errors.New(fmt.Sprintf("the tcp port %s is already used.", key))
				}
			}

			// 添加指定规则到configmap
			if len(tcpProxyMap) > 0 {
				for key, value := range tcpProxyMap {
					tcpConfigMap.Data[key] = value
				}
			}
			// 更新保存config map
			err = r.Update(context.TODO(), tcpConfigMap)
			if err != nil {
				log.Error(err, "failed to update ingress tcp config map.")
				return err
			}
			r.Recorder.Event(tcpConfigMap, "Normal", "SuccessfulUpdate", fmt.Sprintf("SuccessfulUpdate ingress tcp config map for moudle %s  in %s/%s", module.Name, app.Namespace, app.Spec.DisplayName))

			log.Info("successful update ingress config map for tcp.")
		}

		// 更新UDP端口代理
		// 获取 udp config map 中本module中的规则
		udpRules := GetProxyRulesForNameSpaceName(types.NamespacedName{Name: module.Name, Namespace: app.Namespace}, udpConfigMap)
		if !reflect.DeepEqual(udpRules, udpProxyMap) {
			// 如果proxy 发生变化,清空原有配置
			log.Info("the proxy has changed, update ingress config map for udp.")
			for key := range udpRules {
				delete(udpConfigMap.Data, key)
			}

			// 检查端口被占用
			for key := range udpProxyMap {
				_, isExist := udpConfigMap.Data[key]
				if isExist {
					log.Error(nil, "the tcp port is already used.", "protocol", "udp", "port", key)
					return errors.New(fmt.Sprintf("the udp port %s is already used.", key))
				}
			}

			if len(udpProxyMap) > 0 {
				for key, value := range udpProxyMap {
					udpConfigMap.Data[key] = value
				}
			}
			// 更新保存config map
			err = r.Update(context.TODO(), udpConfigMap)
			if err != nil {
				log.Error(err, "failed to update ingress udp config map.")
				r.Recorder.Event(udpConfigMap, "Normal", "SuccessfulUpdate", fmt.Sprintf("SuccessfulUpdate ingress udp config map for moudle %s  in %s/%s", module.Name, app.Namespace, app.Spec.DisplayName))

				return err
			}
			r.Recorder.Event(udpConfigMap, "Normal", "SuccessfulUpdate", fmt.Sprintf("SuccessfulUpdate ingress udp config map for moudle %s  in %s/%s", module.Name, app.Namespace, app.Spec.DisplayName))
			log.Info("successful update ingress config map for udp.")
		}

	}

	return nil
}

func (r *ApplicationReconciler) cleanUpProxy(deploy types.NamespacedName) error {
	// 根据deploy获取当前的tcp/udp configmap
	// 获取tcp configmap
	tcpConfigMap := &corev1.ConfigMap{}
	err := r.Get(context.TODO(), types.NamespacedName{Namespace: KubeSystemNamespace, Name: IngressTCPConfigMap}, tcpConfigMap)
	if err != nil {
		log.Error(err, "failed to get ingress config map for tcp")
		return err
	}

	udpConfigMap := &corev1.ConfigMap{}
	err = r.Get(context.TODO(), types.NamespacedName{Namespace: KubeSystemNamespace, Name: IngressUDPConfigMap}, udpConfigMap)
	if err != nil {
		log.Error(err, "failed to get ingress config map for tcp")
		return err
	}

	// 获取当前deploy的规则
	tcpRules := GetProxyRulesForNameSpaceName(types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, tcpConfigMap)
	udpRules := GetProxyRulesForNameSpaceName(types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}, udpConfigMap)

	// 清空规则并保存
	if len(tcpRules) > 0 {
		for key := range tcpRules {
			delete(tcpConfigMap.Data, key)
		}
	}
	if len(udpRules) > 0 {
		for key := range udpRules {
			delete(udpConfigMap.Data, key)
		}
	}
	err = r.Update(context.TODO(), tcpConfigMap)
	if err != nil {
		log.Error(err, "failed to update ingress tcp config map.")
		return err
	}
	err = r.Update(context.TODO(), udpConfigMap)
	if err != nil {
		log.Error(err, "failed to update ingress udp config map.")
		return err
	}
	return nil
}

func GetProxyRulesForNameSpaceName(nsn types.NamespacedName, confitMap *corev1.ConfigMap) (Rules map[string]string) {
	Rules = make(map[string]string)
	if confitMap == nil || confitMap.Data == nil {
		return
	}
	for cmKey, cmValue := range confitMap.Data {
		cmValuestrs := strings.Split(cmValue, ":")
		namespaceName := types.NamespacedName{
			Name:      nsn.Name,
			Namespace: nsn.Namespace,
		}
		if cmValuestrs[0] == namespaceName.String() {
			Rules[cmKey] = cmValue
		}
	}
	return
}
