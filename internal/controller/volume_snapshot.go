package controller

import (
	"context"
	_ "embed"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed crds/volumesnapshotclasses.yaml
var volumeSnapshotClassCRDYAML []byte

//go:embed crds/volumesnapshots.yaml
var volumeSnapshotCRDYAML []byte

//go:embed crds/volumesnapshotcontents.yaml
var volumeSnapshotContentCRDYAML []byte

func (d *Deployer) applyVolumeSnapshotCRDs(ctx context.Context) error {
	return d.applyCRDs(ctx, [][]byte{volumeSnapshotClassCRDYAML, volumeSnapshotCRDYAML, volumeSnapshotContentCRDYAML})
}

func (d *Deployer) applyUnstructured(ctx context.Context, obj *unstructured.Unstructured) error {
	if d.instance != nil {
		obj.SetOwnerReferences(ownerRefs(d.instance))
	}
	key := client.ObjectKeyFromObject(obj)
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(obj.GroupVersionKind())
	err := d.Client.Get(ctx, key, existing)
	if err == nil {
		obj.SetResourceVersion(existing.GetResourceVersion())
		return d.Client.Update(ctx, obj)
	}
	if !errors.IsNotFound(err) {
		return err
	}
	return d.Client.Create(ctx, obj)
}
