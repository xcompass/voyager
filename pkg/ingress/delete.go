package ingress

import (
	"github.com/appscode/go/errors"
	"github.com/appscode/go/types"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) deleteConfigMap() error {
	err := c.KubeClient.CoreV1().ConfigMaps(c.Ingress.Namespace).Delete(c.Ingress.OffshootName(), &metav1.DeleteOptions{})
	if err != nil && !kerr.IsNotFound(err) {
		return errors.FromErr(err).Err()
	}
	return nil
}

// Ensures deleting all pods if its still exits.
func (c *controller) deletePodsForSelector(selector *metav1.LabelSelector) error {
	r, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return err
	}
	err = c.KubeClient.CoreV1().Pods(c.Ingress.Namespace).DeleteCollection(&metav1.DeleteOptions{
		GracePeriodSeconds: types.Int64P(0),
	}, metav1.ListOptions{
		LabelSelector: r.String(),
	})
	if err != nil && !kerr.IsNotFound(err) {
		return errors.FromErr(err).Err()
	}
	return nil
}

func (c *controller) ensureServiceDeleted() error {
	err := c.KubeClient.CoreV1().Services(c.Ingress.Namespace).Delete(c.Ingress.OffshootName(), &metav1.DeleteOptions{})
	if err != nil && !kerr.IsNotFound(err) {
		return errors.FromErr(err).Err()
	}
	return nil
}

func (c *controller) ensureStatsServiceDeleted() error {
	err := c.KubeClient.CoreV1().Services(c.Ingress.Namespace).Delete(
		c.Ingress.StatsServiceName(),
		&metav1.DeleteOptions{},
	)
	if err != nil && !kerr.IsNotFound(err) {
		return errors.FromErr(err).Err()
	}
	return nil
}

func (c *controller) ensureRBACDeleted() error {
	if err := c.ensureRoleBindingDeleted(); err != nil && !kerr.IsNotFound(err) {
		return errors.FromErr(err).Err()
	}

	if err := c.ensureRolesDeleted(); err != nil && !kerr.IsNotFound(err) {
		return errors.FromErr(err).Err()
	}

	if err := c.ensureServiceAccountDeleted(); err != nil && !kerr.IsNotFound(err) {
		return errors.FromErr(err).Err()
	}
	return nil
}
