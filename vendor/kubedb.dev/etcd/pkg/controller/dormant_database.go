package controller

import (
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_util "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
)

func (c *Controller) WaitUntilPaused(drmn *api.DormantDatabase) error {
	db := &api.Etcd{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drmn.OffshootName(),
			Namespace: drmn.Namespace,
		},
	}

	if err := core_util.WaitUntilPodDeletedBySelector(c.Client, db.Namespace, metav1.SetAsLabelSelector(db.OffshootSelectors())); err != nil {
		return err
	}

	if err := core_util.WaitUntilServiceDeletedBySelector(c.Client, db.Namespace, metav1.SetAsLabelSelector(db.OffshootSelectors())); err != nil {
		return err
	}

	return nil
}

func (c *Controller) WipeOutDatabase(drmn *api.DormantDatabase) error {
	return nil
}

func (c *Controller) deleteMatchingDormantDatabase(etcd *api.Etcd) error {
	// Check if DormantDatabase exists or not
	ddb, err := c.ExtClient.KubedbV1alpha1().DormantDatabases(etcd.Namespace).Get(etcd.Name, metav1.GetOptions{})
	if err != nil {
		if !kerr.IsNotFound(err) {
			return err
		}
		return nil
	}

	// Set WipeOut to false
	if _, _, err := util.PatchDormantDatabase(c.ExtClient.KubedbV1alpha1(), ddb, func(in *api.DormantDatabase) *api.DormantDatabase {
		in.Spec.WipeOut = false
		return in
	}); err != nil {
		return err
	}

	// Delete  Matching dormantDatabase
	if err := c.ExtClient.KubedbV1alpha1().DormantDatabases(etcd.Namespace).Delete(etcd.Name,
		meta_util.DeleteInBackground()); err != nil && !kerr.IsNotFound(err) {
		return err
	}

	return nil
}

func (c *Controller) createDormantDatabase(etcd *api.Etcd) (*api.DormantDatabase, error) {
	dormantDb := &api.DormantDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcd.Name,
			Namespace: etcd.Namespace,
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindEtcd,
			},
		},
		Spec: api.DormantDatabaseSpec{
			Origin: api.Origin{
				ObjectMeta: metav1.ObjectMeta{
					Name:              etcd.Name,
					Namespace:         etcd.Namespace,
					Labels:            etcd.Labels,
					Annotations:       etcd.Annotations,
					CreationTimestamp: etcd.CreationTimestamp,
				},
				Spec: api.OriginSpec{
					Etcd: &etcd.Spec,
				},
			},
		},
	}

	return c.ExtClient.KubedbV1alpha1().DormantDatabases(dormantDb.Namespace).Create(dormantDb)
}
