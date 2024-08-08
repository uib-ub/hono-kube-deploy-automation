package client

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
)

// Kustomizer implements a kustomizer, which is used to build kustomize
// resources from a given source.
type Kustomizer struct {
	KubeSrc string // KubeSrc is the source directory containing kustomize resources.
}

// NewKustomizer returns a new instance of Kustomizer with the provided kubeSrc.
func NewKustomizer(kubeSrc string) *Kustomizer {
	return &Kustomizer{
		KubeSrc: kubeSrc,
	}
}

// Build compiles the kustomize resources into a slice of YAML strings.
// It returns the compiled YAML strings or an error if the build process fails.
func (k *Kustomizer) Build() ([]string, error) {
	log.Infof("Building kustomize resources from %s", k.KubeSrc)
	// Create a filesystem interface for the kustomize to interact with the disk.
	fs := filesys.MakeFsOnDisk()
	// Build compiles the kustomize resources into a slice of YAML strings.
	opts := &krusty.Options{PluginConfig: types.DisabledPluginConfig()}
	// Create a kustomizer instance with the options.
	krust := krusty.MakeKustomizer(opts)
	// Run the kustomizer to build the resources from the source.
	res, err := krust.Run(fs, k.KubeSrc)
	if err != nil {
		return nil, fmt.Errorf("failed to build kustomize resources: %w", err)
	}
	// Initialize a slice to hold the resulting YAML strings.
	allKubeResources := make([]string, 0, len(res.Resources()))
	for _, r := range res.Resources() {
		kubeRes, err := r.AsYAML()
		if err != nil {
			return nil, fmt.Errorf("failed to convert kustomize resource to YAML: %w", err)
		}
		// Append the YAML string to the result slice.
		allKubeResources = append(allKubeResources, string(kubeRes))
	}
	return allKubeResources, nil
}
