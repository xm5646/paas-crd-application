/**
 * 功能描述: 控制器所需的相关工具方法
 * @Date: 2019-11-15
 * @author: lixiaoming
 */
package controllers

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
