// Package contract contains the deployment-neutral shape used by Scheduler to
// contribute its authoring capabilities to a host catalog. Hosts may adapt the
// JSON-compatible value into their own aggregate contract without owning a
// second copy of Scheduler routes, schemas, examples, or execution semantics.
package contract

type CapabilityAuthoringDomain struct {
	Key          string                          `json:"key"`
	Capabilities []CapabilityAuthoringDefinition `json:"capabilities"`
}

type CapabilityAuthoringDefinition struct {
	Key                                     string                                 `json:"key"`
	Status                                  string                                 `json:"status"`
	Lifecycle                               string                                 `json:"lifecycle"`
	AllowedContexts                         []string                               `json:"allowed_contexts,omitempty"`
	Parameters                              []CapabilityAuthoringParameter         `json:"parameters,omitempty"`
	Requires                                []string                               `json:"requires,omitempty"`
	Conflicts                               []string                               `json:"conflicts,omitempty"`
	Permissions                             []string                               `json:"permissions,omitempty"`
	AuditEvents                             []string                               `json:"audit_events,omitempty"`
	ValidationEndpoint                      string                                 `json:"validation_endpoint,omitempty"`
	PreviewEndpoint                         string                                 `json:"preview_endpoint,omitempty"`
	SimulationEndpoint                      string                                 `json:"simulation_endpoint,omitempty"`
	ConfigurationRoutes                     []string                               `json:"configuration_routes,omitempty"`
	ResourceOperations                      *CapabilityAuthoringResourceOperations `json:"resource_operations,omitempty"`
	ResourceKeyPathParameter                string                                 `json:"resource_key_path_parameter,omitempty"`
	SystemDraftResourceType                 string                                 `json:"system_draft_resource_type,omitempty"`
	SystemDraftResourceTypeInputJSONPointer string                                 `json:"system_draft_resource_type_input_json_pointer,omitempty"`
	Errors                                  []CapabilityAuthoringError             `json:"errors,omitempty"`
	Examples                                []CapabilityAuthoringExample           `json:"examples,omitempty"`
	InputSchema                             *CapabilityAuthoringSchema             `json:"input_schema,omitempty"`
	OutputSchema                            *CapabilityAuthoringSchema             `json:"output_schema,omitempty"`
	OutputVariables                         []CapabilityAuthoringOutput            `json:"output_variables,omitempty"`
	ReferenceContracts                      []CapabilityAuthoringReference         `json:"reference_contracts,omitempty"`
	Execution                               *CapabilityAuthoringExecution          `json:"execution,omitempty"`
	Sources                                 []CapabilityAuthoringSource            `json:"sources"`
}

type CapabilityAuthoringResourceOperations struct {
	PersistenceMode string                             `json:"persistence_mode"`
	Validate        string                             `json:"validate"`
	Upsert          string                             `json:"upsert"`
	UpsertHeaders   []CapabilityAuthoringRequestHeader `json:"upsert_headers"`
	SuccessSchema   *CapabilityAuthoringSchema         `json:"success_schema"`
	Get             string                             `json:"get"`
	Versions        string                             `json:"versions"`
	Simulate        string                             `json:"simulate,omitempty"`
	Rollback        string                             `json:"rollback,omitempty"`
	Delete          string                             `json:"delete,omitempty"`
}

type CapabilityAuthoringRequestHeader struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	ValueSource string `json:"value_source"`
	Description string `json:"description"`
}

type CapabilityAuthoringSchema struct {
	Schema               string                               `json:"$schema,omitempty"`
	Ref                  string                               `json:"$ref,omitempty"`
	Type                 string                               `json:"type,omitempty"`
	Properties           map[string]CapabilityAuthoringSchema `json:"properties,omitempty"`
	Definitions          map[string]CapabilityAuthoringSchema `json:"$defs,omitempty"`
	Required             []string                             `json:"required,omitempty"`
	Items                *CapabilityAuthoringSchema           `json:"items,omitempty"`
	OneOf                []CapabilityAuthoringSchema          `json:"oneOf,omitempty"`
	Enum                 []any                                `json:"enum,omitempty"`
	Const                any                                  `json:"const,omitempty"`
	Default              any                                  `json:"default,omitempty"`
	Format               string                               `json:"format,omitempty"`
	Minimum              *float64                             `json:"minimum,omitempty"`
	Maximum              *float64                             `json:"maximum,omitempty"`
	MinLength            *int                                 `json:"minLength,omitempty"`
	MaxLength            *int                                 `json:"maxLength,omitempty"`
	MinItems             *int                                 `json:"minItems,omitempty"`
	MaxItems             *int                                 `json:"maxItems,omitempty"`
	AdditionalProperties *bool                                `json:"additionalProperties,omitempty"`
	Description          string                               `json:"description,omitempty"`
}

type CapabilityAuthoringOutput struct {
	Name        string `json:"name"`
	JSONPointer string `json:"json_pointer"`
	Type        string `json:"type"`
	VisibleTo   string `json:"visible_to"`
}

type CapabilityAuthoringReference struct {
	Kind             string `json:"kind"`
	InputJSONPointer string `json:"input_json_pointer"`
	ScopeFrom        string `json:"scope_from,omitempty"`
	ResolverEndpoint string `json:"resolver_endpoint"`
}

type CapabilityAuthoringExecution struct {
	ReadSet         []string `json:"read_set,omitempty"`
	WriteSet        []string `json:"write_set,omitempty"`
	BoundaryClass   string   `json:"boundary_class"`
	Transaction     string   `json:"transaction"`
	Idempotency     string   `json:"idempotency"`
	SideEffects     []string `json:"side_effects,omitempty"`
	SideEffectLevel string   `json:"side_effect_level"`
	Compensation    string   `json:"compensation,omitempty"`
	PermissionModel string   `json:"permission_model"`
	ChangeControl   string   `json:"change_control,omitempty"`
}

type CapabilityAuthoringExample struct {
	Name               string         `json:"name"`
	Value              map[string]any `json:"value"`
	ExpectedErrorCodes []string       `json:"expected_error_codes,omitempty"`
}

type CapabilityAuthoringParameter struct {
	Key           string         `json:"key"`
	Type          string         `json:"type"`
	Required      bool           `json:"required,omitempty"`
	Default       any            `json:"default,omitempty"`
	Enum          []string       `json:"enum,omitempty"`
	Minimum       *float64       `json:"minimum,omitempty"`
	Maximum       *float64       `json:"maximum,omitempty"`
	MinLength     *int           `json:"min_length,omitempty"`
	MaxLength     *int           `json:"max_length,omitempty"`
	RequiredWhen  map[string]any `json:"required_when,omitempty"`
	ConflictsWith []string       `json:"conflicts_with,omitempty"`
	ItemSchema    string         `json:"item_schema,omitempty"`
	Format        string         `json:"format,omitempty"`
	ReadOnly      bool           `json:"read_only,omitempty"`
}

type CapabilityAuthoringError struct {
	Code          string   `json:"code"`
	FieldPath     string   `json:"field_path,omitempty"`
	ParameterKeys []string `json:"parameter_keys,omitempty"`
	MessageKey    string   `json:"message_key"`
}

type CapabilityAuthoringSource struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Symbol string `json:"symbol,omitempty"`
}
