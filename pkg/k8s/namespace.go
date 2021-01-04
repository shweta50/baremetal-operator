package k8s

/*
 *      Copyright 2020 Platform9, Inc.
 *      All rights reserved
 */

import (
	"context"

	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (w *Watcher) createNSIfNotExists(targetNS string) error {
	_, err := w.client.CoreV1().Namespaces().Get(context.Background(), targetNS, metav1.GetOptions{})
	if err != nil && k8serror.IsNotFound(err) {

		cns := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: targetNS,
			},
		}

		_, err = w.client.CoreV1().Namespaces().Create(context.Background(), &cns, metav1.CreateOptions{})
	}

	return err
}
