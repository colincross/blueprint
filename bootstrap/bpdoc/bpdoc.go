package bpdoc

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"
)

func Packages(pkgFiles map[string][]string, moduleTypeFactories map[string]reflect.Value,
	moduleTypePropertyStructs map[string][]interface{}) ([]*Package, error) {
	r := NewReader(pkgFiles)

	packages := map[string]*Package{}
	var packageList []*Package

	var moduleTypeList []*ModuleType
	for moduleType, propertyStructs := range moduleTypePropertyStructs {
		mt, err := getModuleType(r, moduleType, moduleTypeFactories[moduleType], propertyStructs)
		if err != nil {
			return nil, err
		}
		removeEmptyPropertyStructs(mt)
		collapseDuplicatePropertyStructs(mt)
		collapseNestedPropertyStructs(mt)
		combineDuplicateProperties(mt)

		pkg := packages[mt.PkgPath]
		if pkg == nil {
			var err error
			pkg, err = r.Package(mt.PkgPath)
			if err != nil {
				return nil, err
			}
			packages[mt.PkgPath] = pkg
			packageList = append(packageList, pkg)
		}

		pkg.ModuleTypes = append(pkg.ModuleTypes, mt)
		moduleTypeList = append(moduleTypeList, mt)
	}

	for _, pkg := range packageList {
		sort.Slice(pkg.ModuleTypes, func(i, j int) bool { return pkg.ModuleTypes[i].Name < pkg.ModuleTypes[j].Name })
	}

	sort.Slice(packageList, func(i, j int) bool { return packageList[i].PkgPath < packageList[j].PkgPath })

	return packageList, nil
}

func getModuleType(r *Reader, moduleTypeName string, factory reflect.Value,
	propertyStructs []interface{}) (*ModuleType, error) {

	mt, err := r.ModuleType(moduleTypeName, factory)
	if err != nil {
		return nil, err
	}

	for _, s := range propertyStructs {
		v := reflect.ValueOf(s).Elem()
		t := v.Type()

		// Ignore property structs with unexported or unnamed types
		if t.PkgPath() == "" {
			continue
		}
		ps, err := r.PropertyStruct(t.PkgPath(), t.Name(), v)
		if err != nil {
			return nil, err
		}
		ps.ExcludeByTag("blueprint", "mutated")

		for nestedName, nestedValue := range nestedPropertyStructs(v) {
			nestedType := nestedValue.Type()

			// Ignore property structs with unexported or unnamed types
			if nestedType.PkgPath() == "" {
				continue
			}
			nested, err := r.PropertyStruct(nestedType.PkgPath(), nestedType.Name(), nestedValue)
			if err != nil {
				return nil, err
			}
			nested.ExcludeByTag("blueprint", "mutated")
			nestPoint := ps.GetByName(nestedName)
			if nestPoint == nil {
				return nil, fmt.Errorf("nesting point %q not found", nestedName)
			}

			key, value, err := blueprint.HasFilter(nestPoint.Tag)
			if err != nil {
				return nil, err
			}
			if key != "" {
				nested.IncludeByTag(key, value)
			}

			nestPoint.Nest(nested)
		}
		mt.PropertyStructs = append(mt.PropertyStructs, ps)
	}

	return mt, nil
}

func nestedPropertyStructs(s reflect.Value) map[string]reflect.Value {
	ret := make(map[string]reflect.Value)
	var walk func(structValue reflect.Value, prefix string)
	walk = func(structValue reflect.Value, prefix string) {
		typ := structValue.Type()
		for i := 0; i < structValue.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				// The field is not exported so just skip it.
				continue
			}

			fieldValue := structValue.Field(i)

			switch fieldValue.Kind() {
			case reflect.Bool, reflect.String, reflect.Slice, reflect.Int, reflect.Uint:
				// Nothing
			case reflect.Struct:
				walk(fieldValue, prefix+proptools.PropertyNameForField(field.Name)+".")
			case reflect.Ptr, reflect.Interface:
				if !fieldValue.IsNil() {
					// We leave the pointer intact and zero out the struct that's
					// pointed to.
					elem := fieldValue.Elem()
					if fieldValue.Kind() == reflect.Interface {
						if elem.Kind() != reflect.Ptr {
							panic(fmt.Errorf("can't get type of field %q: interface "+
								"refers to a non-pointer", field.Name))
						}
						elem = elem.Elem()
					}
					if elem.Kind() == reflect.Struct {
						nestPoint := prefix + proptools.PropertyNameForField(field.Name)
						ret[nestPoint] = elem
						walk(elem, nestPoint+".")
					}
				}
			default:
				panic(fmt.Errorf("unexpected kind for property struct field %q: %s",
					field.Name, fieldValue.Kind()))
			}
		}

	}

	walk(s, "")
	return ret
}

// Remove any property structs that have no exported fields
func removeEmptyPropertyStructs(mt *ModuleType) {
	for i := 0; i < len(mt.PropertyStructs); i++ {
		if len(mt.PropertyStructs[i].Properties) == 0 {
			mt.PropertyStructs = append(mt.PropertyStructs[:i], mt.PropertyStructs[i+1:]...)
			i--
		}
	}
}

// Squashes duplicates of the same property struct into single entries
func collapseDuplicatePropertyStructs(mt *ModuleType) {
	var collapsed []*PropertyStruct

propertyStructLoop:
	for _, from := range mt.PropertyStructs {
		for _, to := range collapsed {
			if from.Name == to.Name {
				collapseDuplicateProperties(&to.Properties, &from.Properties)
				continue propertyStructLoop
			}
		}
		collapsed = append(collapsed, from)
	}
	mt.PropertyStructs = collapsed
}

func collapseDuplicateProperties(to, from *[]Property) {
propertyLoop:
	for _, f := range *from {
		for i := range *to {
			t := &(*to)[i]
			if f.Name == t.Name {
				collapseDuplicateProperties(&t.Properties, &f.Properties)
				continue propertyLoop
			}
		}
		*to = append(*to, f)
	}
}

// Find all property structs that only contain structs, and move their children up one with
// a prefixed name
func collapseNestedPropertyStructs(mt *ModuleType) {
	for _, ps := range mt.PropertyStructs {
		collapseNestedProperties(&ps.Properties)
	}
}

func collapseNestedProperties(p *[]Property) {
	var n []Property

	for _, parent := range *p {
		var containsProperty bool
		for j := range parent.Properties {
			child := &parent.Properties[j]
			if len(child.Properties) > 0 {
				collapseNestedProperties(&child.Properties)
			} else {
				containsProperty = true
			}
		}
		if containsProperty || len(parent.Properties) == 0 {
			n = append(n, parent)
		} else {
			for j := range parent.Properties {
				child := parent.Properties[j]
				child.Name = parent.Name + "." + child.Name
				n = append(n, child)
			}
		}
	}
	*p = n
}

func combineDuplicateProperties(mt *ModuleType) {
	for _, ps := range mt.PropertyStructs {
		combineDuplicateSubProperties(&ps.Properties)
	}
}

func combineDuplicateSubProperties(p *[]Property) {
	var n []Property
propertyLoop:
	for _, child := range *p {
		if len(child.Properties) > 0 {
			combineDuplicateSubProperties(&child.Properties)
			for i := range n {
				s := &n[i]
				if s.SameSubProperties(child) {
					s.OtherNames = append(s.OtherNames, child.Name)
					s.OtherTexts = append(s.OtherTexts, child.Text)
					continue propertyLoop
				}
			}
		}
		n = append(n, child)
	}

	*p = n
}

type moduleTypeByName []*ModuleType

func (l moduleTypeByName) Len() int           { return len(l) }
func (l moduleTypeByName) Less(i, j int) bool { return l[i].Name < l[j].Name }
func (l moduleTypeByName) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

// Package contains the information about a package relevant to generating documentation.
type Package struct {
	// Name is the name of the package.
	Name string

	// PkgPath is the full package path of the package as used in the primary builder.
	PkgPath string

	// Text is the contents of the package comment documenting the module types in the package.
	Text string

	// ModuleTypes is a list of ModuleType objects that contain information about each module type that is
	// defined by the package.
	ModuleTypes []*ModuleType
}

// ModuleType contains the information about a module type that is relevant to generating documentation.
type ModuleType struct {
	// Name is the string that will appear in Blueprints files when defining a new module of
	// this type.
	Name string

	// PkgPath is the full package path of the package that contains the module type factory.
	PkgPath string

	// Text is the contents of the comment documenting the module type.
	Text string

	// PropertyStructs is a list of PropertyStruct objects that contain information about each
	// property struct that is used by the module type, containing all properties that are valid
	// for the module type.
	PropertyStructs []*PropertyStruct
}
