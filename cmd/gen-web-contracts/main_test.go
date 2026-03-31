package main

import (
	"reflect"
	"strings"
	"testing"

	"codeberg.org/aros/dpview/internal/api"
)

func TestStructSchemaUsesStrictObjectForGeneratedContracts(t *testing.T) {
	g := newGenerator()

	schema := g.structSchema(reflect.TypeOf(api.Error{}), reflect.TypeOf(api.Error{}), false)

	if !strings.Contains(schema, "z.strictObject({") {
		t.Fatalf("structSchema() = %q, want strict object schema", schema)
	}
	if strings.Contains(schema, "z.looseObject({") {
		t.Fatalf("structSchema() = %q, should not allow loose object schema", schema)
	}
}

func TestRenderDoesNotEmitLooseObjectSchemas(t *testing.T) {
	g := newGenerator()
	g.registerRoots(reflect.TypeOf(api.Error{}), reflect.TypeOf(api.SettingsData{}))

	output, err := g.render()
	if err != nil {
		t.Fatalf("render() error = %v", err)
	}

	rendered := string(output)
	if strings.Contains(rendered, "z.looseObject(") {
		t.Fatalf("render() emitted loose object schema:\n%s", rendered)
	}
	if !strings.Contains(rendered, "export const apiErrorSchema: z.ZodType<ApiError> = z.strictObject({") {
		t.Fatalf("render() missing strict ApiError schema:\n%s", rendered)
	}
	if !strings.Contains(rendered, "export const settingsDataSchema: z.ZodType<SettingsData> = z.strictObject({") {
		t.Fatalf("render() missing strict SettingsData schema:\n%s", rendered)
	}
}
