package main

import (
	"fmt"
	"os"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: crd-breaking-check <old-crd.yaml> <new-crd.yaml>\n")
		os.Exit(2)
	}

	oldCRD, err := loadCRD(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load old CRD: %v\n", err)
		os.Exit(1)
	}

	newCRD, err := loadCRD(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load new CRD: %v\n", err)
		os.Exit(1)
	}

	var breaking []string

	for _, newVer := range newCRD.Spec.Versions {
		oldVer := findVersion(oldCRD, newVer.Name)
		if oldVer == nil {
			continue
		}
		if newVer.Schema == nil || newVer.Schema.OpenAPIV3Schema == nil {
			continue
		}
		if oldVer.Schema == nil || oldVer.Schema.OpenAPIV3Schema == nil {
			continue
		}
		breaking = append(breaking, compareSchemas(
			oldVer.Name,
			"",
			*oldVer.Schema.OpenAPIV3Schema,
			*newVer.Schema.OpenAPIV3Schema,
		)...)
	}

	if len(breaking) > 0 {
		fmt.Println("BREAKING CHANGES DETECTED:")
		for _, b := range breaking {
			fmt.Printf("  - %s\n", b)
		}
		os.Exit(1)
	}
	fmt.Println("No breaking changes detected.")
}

func loadCRD(path string) (*apiextensionsv1.CustomResourceDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096).Decode(&crd); err != nil {
		return nil, err
	}
	return &crd, nil
}

func findVersion(crd *apiextensionsv1.CustomResourceDefinition, name string) *apiextensionsv1.CustomResourceDefinitionVersion {
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Name == name {
			return &crd.Spec.Versions[i]
		}
	}
	return nil
}

func compareSchemas(version string, prefix string, oldSchema, newSchema apiextensionsv1.JSONSchemaProps) []string {
	var breaking []string

	if oldSchema.Type != newSchema.Type {
		breaking = append(breaking, fmt.Sprintf("[%s] %s: type changed from %q to %q", version, prefix, oldSchema.Type, newSchema.Type))
	}

	oldRequired := toSet(oldSchema.Required)
	newRequired := toSet(newSchema.Required)
	for field := range newRequired {
		if !oldRequired[field] {
			breaking = append(breaking, fmt.Sprintf("[%s] %s.%s: field became required (was optional)", version, prefix, field))
		}
	}
	for field := range oldRequired {
		if !newRequired[field] {
			breaking = append(breaking, fmt.Sprintf("[%s] %s.%s: field became optional (was required)", version, prefix, field))
		}
	}
	if len(newSchema.Enum) > 0 {
		oldEnum := toSetEnum(oldSchema.Enum)
		for _, v := range newSchema.Enum {
			if !oldEnum[string(v.Raw)] {
				breaking = append(breaking, fmt.Sprintf("[%s] %s: added enum value %q", version, prefix, string(v.Raw)))
			}
		}
		for _, v := range oldSchema.Enum {
			if !containsEnum(newSchema.Enum, v) {
				breaking = append(breaking, fmt.Sprintf("[%s] %s: removed enum value %q", version, prefix, string(v.Raw)))
			}
		}
	}

	allProps := mergeKeys(oldSchema.Properties, newSchema.Properties)
	for propName := range allProps {
		oldProp, oldOK := oldSchema.Properties[propName]
		newProp, newOK := newSchema.Properties[propName]
		propPath := propName
		if prefix != "" {
			propPath = prefix + "." + propName
		}
		if !newOK {
			breaking = append(breaking, fmt.Sprintf("[%s] %s: property removed", version, propPath))
			continue
		}
		if !oldOK {
			continue
		}
		breaking = append(breaking, compareSchemas(version, propPath, oldProp, newProp)...)
	}

	return breaking
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}

func toSetEnum(items []apiextensionsv1.JSON) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[string(item.Raw)] = true
	}
	return m
}

func containsEnum(items []apiextensionsv1.JSON, target apiextensionsv1.JSON) bool {
	for _, item := range items {
		if string(item.Raw) == string(target.Raw) {
			return true
		}
	}
	return false
}

func mergeKeys(a, b map[string]apiextensionsv1.JSONSchemaProps) map[string]bool {
	m := make(map[string]bool, len(a)+len(b))
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	return m
}
