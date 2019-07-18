package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"sort"
	"strings"
)

type state struct {
	Modules []moduleState `json:"modules"`
}

// read populates the state object from a statefile.
func (s *state) read(stateFile io.Reader) error {

	// read statefile contents
	b, err := ioutil.ReadAll(stateFile)
	if err != nil {
		return err
	}

	// when calling terraform-inventory from terraform (i.e. inventory is not fully formed yet),
	// o: is prefixing the json payload forming the state. I have no idea why yet...
	// I'm just stripping the chars, expecting a proper start of dict symbol (i.e. {).
	i := 0
	for ; i < len(b) && b[i] != '{'; i++ {}

	// parse into struct
	err = json.Unmarshal(b[i:len(b)], s)
	if err != nil {
		return err
	}

	return nil
}

// outputs returns a slice of the Outputs found in the statefile.
func (s *state) outputs() []*Output {
	inst := make([]*Output, 0)

	for _, m := range s.Modules {
		for k, v := range m.Outputs {
			var o *Output
			switch v := v.(type) {
			case map[string]interface{}:
				o, _ = NewOutput(k, v["value"])
			case string:
				o, _ = NewOutput(k, v)
			default:
				o, _ = NewOutput(k, "<error>")
			}

			inst = append(inst, o)
		}
	}

	return inst
}

// map of resource ID -> resource Name
func (s *state) mapResourceIDNames() map[string]string {
	t := map[string]string{}

	for _, m := range s.Modules {
		for _, k := range m.resourceKeys() {
			if m.ResourceStates[k].Primary.ID != "" && m.ResourceStates[k].Primary.Attributes["name"] != "" {
				kk := strings.ToLower(m.ResourceStates[k].Primary.ID)
				t[kk] = m.ResourceStates[k].Primary.Attributes["name"]
			}
		}
	}
	return t
}

// resources returns a slice of the Resources found in the statefile.
func (s *state) resources() []*Resource {
	inst := make([]*Resource, 0)

	for _, m := range s.Modules {
		for _, k := range m.resourceKeys() {
			// Terraform stores resources in a name->map map, but we need the name to
			// decide which groups to include the resource in. So wrap it in a higher-
			// level object with both properties.
			r, err := NewResource(k, m.ResourceStates[k])
			if err != nil {
				continue
			}
			if r.IsSupported() {
				inst = append(inst, r)
			}
		}
	}

	return inst
}

type moduleState struct {
	ResourceStates map[string]resourceState `json:"resources"`
	Outputs        map[string]interface{}   `json:"outputs"`
}

// resourceKeys returns a sorted slice of the key names of the resources in this
// module. Do this instead of range over ResourceStates, to ensure that the
// output is consistent.
func (ms *moduleState) resourceKeys() []string {
	lk := len(ms.ResourceStates)
	keys := make([]string, lk, lk)
	i := 0

	for k := range ms.ResourceStates {
		keys[i] = k
		i += 1
	}

	sort.Strings(keys)
	return keys
}

type resourceState struct {

	// Populated from statefile
	Type    string        `json:"type"`
	Primary instanceState `json:"primary"`
}

type instanceState struct {
	ID         string            `json:"id"`
	Attributes map[string]string `json:"attributes,omitempty"`
}
