package client

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
)

type Kustomizer struct {
	KubeSrc string
}

func NewKustomizer(kubeSrc string) *Kustomizer {
	return &Kustomizer{
		KubeSrc: kubeSrc,
	}
}

// Build compiles the kustomize resources into a slice of YAML strings.
func (k *Kustomizer) Build() ([]string, error) {
	log.Infof("Building kustomize resources from %s", k.KubeSrc)
	fs := filesys.MakeFsOnDisk()
	opt := &krusty.Options{PluginConfig: types.DisabledPluginConfig()}
	krust := krusty.MakeKustomizer(opt)
	res, err := krust.Run(fs, k.KubeSrc)
	if err != nil {
		return nil, fmt.Errorf("failed to build kustomize resources: %w", err)
	}
	allKubeResources := make([]string, 0, len(res.Resources()))
	for _, r := range res.Resources() {
		kubeRes, err := r.AsYAML()
		if err != nil {
			return nil, fmt.Errorf("failed to convert kustomize resource to YAML: %w", err)
		}
		allKubeResources = append(allKubeResources, string(kubeRes))
	}
	return allKubeResources, nil
}
