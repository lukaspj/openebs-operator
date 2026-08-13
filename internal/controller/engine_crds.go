package controller

import (
	"context"
	_ "embed"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

//go:embed crds/lvmnode.yaml
var lvmNodeCRDYAML []byte

//go:embed crds/lvmvolume.yaml
var lvmVolumeCRDYAML []byte

//go:embed crds/lvmsnapshot.yaml
var lvmSnapshotCRDYAML []byte

//go:embed crds/zfsnode.yaml
var zfsNodeCRDYAML []byte

//go:embed crds/zfsvolume.yaml
var zfsVolumeCRDYAML []byte

//go:embed crds/zfssnapshot.yaml
var zfsSnapshotCRDYAML []byte

func (d *Deployer) applyCRDs(ctx context.Context, yamlBytes [][]byte) error {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	for _, yb := range yamlBytes {
		obj := &unstructured.Unstructured{}
		if _, _, err := decoder.Decode(yb, nil, obj); err != nil {
			return err
		}
		if err := d.applyUnstructured(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

func (d *Deployer) applyLVMCRDs(ctx context.Context) error {
	return d.applyCRDs(ctx, [][]byte{lvmNodeCRDYAML, lvmVolumeCRDYAML, lvmSnapshotCRDYAML})
}

func (d *Deployer) applyZFSCRDs(ctx context.Context) error {
	return d.applyCRDs(ctx, [][]byte{zfsNodeCRDYAML, zfsVolumeCRDYAML, zfsSnapshotCRDYAML})
}
