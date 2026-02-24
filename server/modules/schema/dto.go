package schema

import "encoding/json"

// SchemaListResponse represents the response for listing all schemas
type SchemaListResponse struct {
	Schemas []SchemaInfo `json:"schemas"`
}

// SchemaInfo represents basic information about a schema
type SchemaInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Module      string `json:"module"`
}

// SchemaDetailResponse represents the complete schema structure
type SchemaDetailResponse struct {
	Schema      string               `json:"$schema"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	SoftDelete  bool                 `json:"softDelete,omitempty"`
	Info        *SchemaMetaInfo      `json:"info,omitempty"`
	UI          *SchemaUI            `json:"ui,omitempty"`
	Features    *SchemaFeatures      `json:"features,omitempty"`
	Properties  map[string]*Property `json:"properties"`
}

// SchemaMetaInfo contains metadata about the schema
type SchemaMetaInfo struct {
	CollectionName string `json:"collectionName"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description"`
	Icon           string `json:"icon"`
	Locale         string `json:"locale"`
}

// SchemaUI contains UI configuration for forms
type SchemaUI struct {
	SubmitText string    `json:"submitText,omitempty"`
	ResetText  string    `json:"resetText,omitempty"`
	ShowReset  bool      `json:"showReset,omitempty"`
	Layout     *UILayout `json:"layout,omitempty"`
}

// UILayout defines the form layout
type UILayout struct {
	Direction string `json:"direction,omitempty"`
	Gap       int    `json:"gap,omitempty"`
	Columns   int    `json:"columns,omitempty"`
}

// SchemaFeatures defines feature flags for the schema
type SchemaFeatures struct {
	SoftDelete bool `json:"softDelete,omitempty"`
	Export     bool `json:"export,omitempty"`
	Import     bool `json:"import,omitempty"`
	Batch      bool `json:"batch,omitempty"`
}

// Property represents a field in the schema
type Property struct {
	Type        string           `json:"type"`
	Label       string           `json:"label"`
	Description string           `json:"description,omitempty"`
	Default     any              `json:"default,omitempty"`
	Private     bool             `json:"private,omitempty"`
	Writable    *bool            `json:"writable,omitempty"`
	Unique      bool             `json:"unique,omitempty"`
	Queryable   bool             `json:"queryable,omitempty"`
	Exportable  bool             `json:"exportable,omitempty"`
	Validate    *ValidationRules `json:"validate,omitempty"`
	UI          *PropertyUI      `json:"ui,omitempty"`
	Ref         string           `json:"$ref,omitempty"`
	Relation    *RelationConfig  `json:"$relation,omitempty"`
}

// ValidationRules defines validation constraints
type ValidationRules struct {
	Required bool     `json:"required,omitempty"`
	Min      *int     `json:"min,omitempty"`
	Max      *int     `json:"max,omitempty"`
	Format   string   `json:"format,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

// PropertyUI defines UI configuration for a property
type PropertyUI struct {
	Widget      string         `json:"widget,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	Span        int            `json:"span,omitempty"`
	ShowInList  bool           `json:"showInList,omitempty"`
	ShowInForm  bool           `json:"showInForm,omitempty"`
	Sortable    bool           `json:"sortable,omitempty"`
	Filterable  bool           `json:"filterable,omitempty"`
	Multiple    bool           `json:"multiple,omitempty"`
	Options     []SelectOption `json:"options,omitempty"`
}

// SelectOption represents an option in a select widget
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// RelationConfig defines relationship configuration
type RelationConfig struct {
	Type       string `json:"type"`
	LabelField string `json:"labelField,omitempty"`
}

// UnmarshalJSON custom unmarshaler to handle dynamic property types
func (p *Property) UnmarshalJSON(data []byte) error {
	type Alias Property
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}
