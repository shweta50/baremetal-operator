/* Helper package to merge existing Objects with ones we are trying to create
 * Shamelessly copied from SRIOV Network Operator
 */

package apply

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteObject deletes the desired object against the apiserver,
func DeleteObject(ctx context.Context, client k8sclient.Client, obj *uns.Unstructured) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	if name == "" {
		return errors.Errorf("Object %s has no name", obj.GroupVersionKind().String())
	}
	gvk := obj.GroupVersionKind()
	// used for logging and errors
	objDesc := fmt.Sprintf("(%s) %s/%s", gvk.String(), namespace, name)
	//log.Infof("reconciling %s", objDesc)

	if err := IsObjectSupported(obj); err != nil {
		return errors.Wrapf(err, "object %s unsupported", objDesc)
	}

	// Get existing
	existing := &uns.Unstructured{}
	existing.SetGroupVersionKind(gvk)
	err := client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)

	if err != nil && apierrors.IsNotFound(err) {
		log.Infof("does not exist, do nothing %s", objDesc)
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "could not retrieve existing %s", objDesc)
	}

	if err = client.Delete(ctx, existing); err != nil {
		return errors.Wrapf(err, "could not delete object %s", objDesc)
	} else {
		log.Infof("delete was successful")
	}
	return nil
}

// ApplyObject applies the desired object against the apiserver,
// merging it with any existing objects if already present.
func ApplyObject(ctx context.Context, client k8sclient.Client, obj *uns.Unstructured) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	if name == "" {
		return errors.Errorf("Object %s has no name", obj.GroupVersionKind().String())
	}
	gvk := obj.GroupVersionKind()
	// used for logging and errors
	objDesc := fmt.Sprintf("(%s) %s/%s", gvk.String(), namespace, name)
	//log.Infof("reconciling %s", objDesc)

	if err := IsObjectSupported(obj); err != nil {
		return errors.Wrapf(err, "object %s unsupported", objDesc)
	}

	// Get existing
	existing := &uns.Unstructured{}
	existing.SetGroupVersionKind(gvk)
	err := client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)

	if err != nil && apierrors.IsNotFound(err) {
		log.Infof("does not exist, creating %s", objDesc)
		err := client.Create(ctx, obj)
		if err != nil {
			return errors.Wrapf(err, "could not create %s", objDesc)
		}
		log.Infof("successfully created %s", objDesc)
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "could not retrieve existing %s", objDesc)
	}

	// Merge the desired object with what actually exists
	if err := MergeObjectForUpdate(existing, obj); err != nil {
		return errors.Wrapf(err, "could not merge object %s with existing", objDesc)
	}
	if !equality.Semantic.DeepDerivative(obj, existing) {
		if err := client.Update(ctx, obj); err != nil {
			return errors.Wrapf(err, "could not update object %s", objDesc)
		} else {
			log.Infof("update was successful")
		}
	} else {
		log.Infof("no change from existing state %s", objDesc)
	}

	return nil
}
