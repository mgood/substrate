package activityspec

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/ajbouh/substrate/images/substrate/fs"
)

type ServiceDef struct {
	Name       string                 `json:"name"`
	Spawn      ServiceDefSpawn        `json:"spawn"`
	Activities map[string]ActivityDef `json:"activities"`
}

type ServiceSpawnRequest struct {
	ServiceName string
	Parameters  ServiceSpawnParameterRequests

	URLPrefix     string
	User          string
	Ephemeral     bool
	ForceReadOnly bool
}

type ServiceSpawnResolution struct {
	User      string
	Ephemeral bool

	ServiceName        string
	Parameters         ServiceSpawnParameters `json:"parameters"`
	GracePeriodSeconds *int                   `json:"grace_period_seconds,omitempty"`

	ServiceDefSpawn ServiceDefSpawn `json:"spawn"`
}

type ServiceSpawnResponse struct {
	Name        string
	BackendURL  string
	BearerToken *string

	ServiceSpawnResolution ServiceSpawnResolution
}

func (s *ServiceSpawnResponse) URL() (*url.URL, error) {
	return url.Parse(s.BackendURL)
}

type ServiceSpawnParameterType string

const ServiceSpawnParameterTypeString ServiceSpawnParameterType = "string"
const ServiceSpawnParameterTypeSpace ServiceSpawnParameterType = "space"
const ServiceSpawnParameterTypeSpaces ServiceSpawnParameterType = "spaces"
const ServiceSpawnParameterTypeResource ServiceSpawnParameterType = "resource"

type ServiceSpawnParameterSchema struct {
	Type        ServiceSpawnParameterType `json:"type"`
	Value       string                    `json:"value,omitempty"`
	Description string                    `json:"description,omitempty"`
	Optional    bool                      `json:"optional,omitempty"`
}

type ServiceDefSpawn struct {
	Image        string                    `json:"image"`
	Environment  map[string]string         `json:"environment,omitempty"`
	Command      []string                  `json:"command,omitempty"`
	ResourceDirs map[string]ResourceDirDef `json:"resourcedirs,omitempty"`
	Mounts       []ServiceDefSpawnMount    `json:"mounts,omitempty"`

	URLPrefix string `json:"url_prefix,omitempty"`
}

type ServiceDefSpawnMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode",omitempty`
}

type ResourceDirDef struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`

	// TODO enough information to fetch at runtime should be included as fields here...
}

type ServiceSpawnParameterRequest string

func (v ServiceSpawnParameterRequest) String() string {
	return string(v)
}

func (v ServiceSpawnParameterRequest) IsConcrete(t ServiceSpawnParameterType) bool {
	switch t {
	case ServiceSpawnParameterTypeSpace:
		return string(v) != "" && !strings.HasPrefix(string(v), spaceViewForkPrefix)
	case ServiceSpawnParameterTypeSpaces:
		for s := range v.Spaces(false) {
			if string(s) == "" || strings.HasPrefix(string(s), spaceViewForkPrefix) {
				return false
			}
		}
	}

	return true
}

func (v ServiceSpawnParameterRequest) Resource() *Resource {
	var unit string
	quantityRunes := []rune{}
	for i, r := range v {
		if r >= '0' || r <= '9' {
			quantityRunes = append(quantityRunes, r)
		} else {
			unit = string(v)[i:]
			break
		}
	}

	quantity := uint64(1)
	if len(quantityRunes) > 0 {
		quantity, _ = strconv.ParseUint(string(quantityRunes), 10, 64)
	}
	return &Resource{
		Unit:     unit,
		Quantity: quantity,
	}
}

func (v ServiceSpawnParameterRequest) Space(forceReadOnly bool) *SpaceViewRequest {
	return ParseViewRequest(string(v), forceReadOnly)
}

func (v ServiceSpawnParameterRequest) Spaces(forceReadOnly bool) []SpaceViewRequest {
	split := strings.Split(string(v), spaceViewMultiSep)
	multi := []SpaceViewRequest{}
	for _, m := range split {
		if m == "" {
			continue
		}
		multi = append(multi, *ParseViewRequest(m, forceReadOnly))
	}

	return multi
}

type ServiceSpawnParameterRequests map[string]ServiceSpawnParameterRequest

type ServiceSpawnParameters map[string]*ServiceSpawnParameter

type Resource struct {
	Unit     string
	Quantity uint64
}

type ServiceSpawnParameter struct {
	String   *string
	Resource *Resource
	Space    *substratefs.SpaceView
	Spaces   *[]substratefs.SpaceView

	Implicit bool
}

func fmtSpaceView(v *substratefs.SpaceView) string {
	suffix := ""
	if v.IsReadOnly {
		suffix = ":ro"
	}
	return v.Tip.String() + suffix
}

func (p *ServiceSpawnParameter) Format() string {
	switch {
	case p.Resource != nil:
		return strconv.FormatUint(p.Resource.Quantity, 10) + p.Resource.Unit
	case p.String != nil:
		return *p.String
	case p.Space != nil:
		space := p.Space
		return fmtSpaceView(space)
	case p.Spaces != nil:
		multi := []string{""} // an initial empty value to say this is a "multi"
		for _, m := range *p.Spaces {
			multi = append(multi, fmtSpaceView(&m))
		}
		// Canonicalize multi order
		sort.Strings(multi)
		return strings.Join(multi, spaceViewMultiSep)
	}

	return ""
}

const spaceViewCut = "="
const spaceViewsSep = ";"
const spaceViewMultiSep = ","
const viewspecParameterStart = "["
const viewspecParameterEnd = "]"

func ParseServiceSpawnRequest(spec string, forceReadOnly bool, spawnPrefix string) (*ServiceSpawnRequest, string, error) {
	var lens string
	var viewspec string
	var path string
	if strings.HasPrefix(spec, viewspecParameterStart) { // lens is unknown!
		viewspec = strings.TrimPrefix(spec, viewspecParameterStart)
		if !strings.HasSuffix(viewspec, viewspecParameterEnd) {
			split := strings.SplitN(viewspec, "/", 2)
			if len(split) > 1 && !strings.HasSuffix(split[0], viewspecParameterEnd) {
				return nil, "", fmt.Errorf("bad spec: %q; viewspec=%q split=%#v", spec, viewspec, split)
			}

			viewspec = split[0]
			path = "/" + split[1]
		}
		viewspec = strings.TrimSuffix(viewspec, viewspecParameterEnd)
	} else {
		var found bool
		lens, viewspec, found = strings.Cut(spec, viewspecParameterStart)
		if found {
			if !strings.HasSuffix(viewspec, viewspecParameterEnd) {
				split := strings.SplitN(viewspec, "/", 2)
				if len(split) > 1 && !strings.HasSuffix(split[0], viewspecParameterEnd) {
					return nil, "", fmt.Errorf("bad spec: %q; viewspec=%q split=%#v", spec, viewspec, split)
				}

				viewspec = split[0]
				path = "/" + split[1]
			}
			viewspec = strings.TrimSuffix(viewspec, viewspecParameterEnd)
		}
	}

	params := ServiceSpawnParameterRequests{}
	if viewspec != "" {
		for _, fragment := range strings.Split(viewspec, spaceViewsSep) {
			k, v, ok := strings.Cut(fragment, spaceViewCut)
			if !ok {
				v = k
				k = "data"
			}
			params[k] = ServiceSpawnParameterRequest(v)
		}
	}

	r := &ServiceSpawnRequest{
		ServiceName: lens,
		Parameters:  params,
		URLPrefix:   spawnPrefix,
	}

	fmt.Printf("ParseServiceSpawnRequest %q %#v\n", spec, *r)

	return r, path, nil
}

func (r *ServiceSpawnRequest) Format() (string, bool) {
	concrete := r.ServiceName != ""

	fragments := []string{}
	params := r.Parameters
	for k := range r.Parameters {
		if k == "" {
			continue
		}
		// TODO consider canonicalizing spaces order...
		fragments = append(fragments, k+spaceViewCut+params[k].String())
	}
	// Canonicalize mount order
	sort.Strings(fragments)

	viewspec := strings.Join(fragments, spaceViewsSep)
	fmt.Printf("ServiceSpawn() viewspec=%q fragments=%#v r=%#v\n", viewspec, fragments, r)

	return r.ServiceName + viewspecParameterStart + viewspec + viewspecParameterEnd, concrete
}

func (r ServiceSpawnResolution) Format() (string, bool) {
	spaceFragments := []string{}
	for k, v := range r.Parameters {
		if v.Implicit {
			continue
		}
		spaceFragments = append(spaceFragments, k+spaceViewCut+v.Format())
	}
	// Canonicalize mount order
	sort.Strings(spaceFragments)

	viewspec := strings.Join(spaceFragments, spaceViewsSep)
	fmt.Printf("ServiceSpawnResolution() viewspec=%s r=%#v\n", viewspec, r)

	return r.ServiceName + viewspecParameterStart + viewspec + viewspecParameterEnd, r.ServiceName != ""
}