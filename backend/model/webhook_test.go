package model

import (
	"reflect"
	"testing"

	"gorm.io/gorm"
)

// TestWebhookConfigStructFields validates that WebhookConfig has the expected
// fields with correct types and GORM tags.
func TestWebhookConfigStructFields(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		wantKind reflect.Kind
		wantTag  string
	}{
		{
			name:     "primary key",
			field:    "ID",
			wantKind: reflect.Uint,
			wantTag:  `gorm:"primarykey" json:"id"`,
		},
		{
			name:     "created_at",
			field:    "CreatedAt",
			wantKind: reflect.Struct,
			wantTag:  `json:"created_at"`,
		},
		{
			name:     "updated_at",
			field:    "UpdatedAt",
			wantKind: reflect.Struct,
			wantTag:  `json:"updated_at"`,
		},
		{
			name:     "deleted_at",
			field:    "DeletedAt",
			wantKind: reflect.Struct,
			wantTag:  `gorm:"index" json:"-"`,
		},
		{
			name:     "tenant_id",
			field:    "TenantID",
			wantKind: reflect.Uint,
			wantTag:  `gorm:"index;not null" json:"tenant_id"`,
		},
		{
			name:     "platform",
			field:    "Platform",
			wantKind: reflect.String,
			wantTag:  `gorm:"size:50;not null" json:"platform"`,
		},
		{
			name:     "webhook_url",
			field:    "WebhookURL",
			wantKind: reflect.String,
			wantTag:  `gorm:"size:500;not null" json:"webhook_url"`,
		},
		{
			name:     "enabled",
			field:    "Enabled",
			wantKind: reflect.Bool,
			wantTag:  `gorm:"default:true" json:"enabled"`,
		},
		{
			name:     "template_down",
			field:    "TemplateDown",
			wantKind: reflect.String,
			wantTag:  `gorm:"type:text" json:"template_down"`,
		},
		{
			name:     "template_up",
			field:    "TemplateUp",
			wantKind: reflect.String,
			wantTag:  `gorm:"type:text" json:"template_up"`,
		},
		{
			name:     "interval_seconds",
			field:    "IntervalSeconds",
			wantKind: reflect.Int,
			wantTag:  `gorm:"default:300" json:"interval_seconds"`,
		},
		{
			name:     "cooldown_minutes",
			field:    "CooldownMinutes",
			wantKind: reflect.Int,
			wantTag:  `gorm:"default:10" json:"cooldown_minutes"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(WebhookConfig{})
			f, ok := typ.FieldByName(tt.field)
			if !ok {
				t.Fatalf("WebhookConfig missing field: %s", tt.field)
			}
			if f.Type.Kind() != tt.wantKind {
				t.Fatalf("WebhookConfig.%s kind = %v, want %v", tt.field, f.Type.Kind(), tt.wantKind)
			}
			gotTag := string(f.Tag)
			if gotTag != tt.wantTag {
				t.Fatalf("WebhookConfig.%s tag = %q, want %q", tt.field, gotTag, tt.wantTag)
			}
		})
	}
}

// TestWebhookConfigTableName verifies TableName returns the expected table name.
func TestWebhookConfigTableName(t *testing.T) {
	want := "webhook_configs"
	got := WebhookConfig{}.TableName()
	if got != want {
		t.Fatalf("WebhookConfig.TableName() = %q, want %q", got, want)
	}
}

// TestWebhookConfigFieldCount is a safety check: if a new field is added to
// WebhookConfig, this test fails and reminds the developer to update the
// table-driven tests above.
func TestWebhookConfigFieldCount(t *testing.T) {
	typ := reflect.TypeOf(WebhookConfig{})
	if got := typ.NumField(); got != 9 {
		t.Fatalf("WebhookConfig has %d fields, expected 9 — add new field tests to TestWebhookConfigStructFields", got)
	}
}

// TestWebhookLogStructFields validates that WebhookLog has the expected
// fields with correct types and GORM tags.
func TestWebhookLogStructFields(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		wantKind reflect.Kind
		wantTag  string
	}{
		{
			name:     "primary key",
			field:    "ID",
			wantKind: reflect.Uint,
			wantTag:  `gorm:"primarykey" json:"id"`,
		},
		{
			name:     "created_at",
			field:    "CreatedAt",
			wantKind: reflect.Struct,
			wantTag:  `json:"created_at"`,
		},
		{
			name:     "updated_at",
			field:    "UpdatedAt",
			wantKind: reflect.Struct,
			wantTag:  `json:"updated_at"`,
		},
		{
			name:     "deleted_at",
			field:    "DeletedAt",
			wantKind: reflect.Struct,
			wantTag:  `gorm:"index" json:"-"`,
		},
		{
			name:     "tenant_id",
			field:    "TenantID",
			wantKind: reflect.Uint,
			wantTag:  `gorm:"index;not null" json:"tenant_id"`,
		},
		{
			name:     "platform",
			field:    "Platform",
			wantKind: reflect.String,
			wantTag:  `gorm:"size:50;not null" json:"platform"`,
		},
		{
			name:     "model_name",
			field:    "ModelName",
			wantKind: reflect.String,
			wantTag:  `gorm:"size:100" json:"model_name"`,
		},
		{
			name:     "status",
			field:    "Status",
			wantKind: reflect.String,
			wantTag:  `gorm:"size:50;not null" json:"status"`,
		},
		{
			name:     "message",
			field:    "Message",
			wantKind: reflect.String,
			wantTag:  `gorm:"type:text" json:"message"`,
		},
		{
			name:     "success",
			field:    "Success",
			wantKind: reflect.Bool,
			wantTag:  `gorm:"not null;default:false" json:"success"`,
		},
		{
			name:     "response_body",
			field:    "ResponseBody",
			wantKind: reflect.String,
			wantTag:  `gorm:"type:longtext" json:"response_body"`,
		},
		{
			name:     "cooldown_skipped",
			field:    "CooldownSkipped",
			wantKind: reflect.Bool,
			wantTag:  `gorm:"default:false" json:"cooldown_skipped"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(WebhookLog{})
			f, ok := typ.FieldByName(tt.field)
			if !ok {
				t.Fatalf("WebhookLog missing field: %s", tt.field)
			}
			if f.Type.Kind() != tt.wantKind {
				t.Fatalf("WebhookLog.%s kind = %v, want %v", tt.field, f.Type.Kind(), tt.wantKind)
			}
			gotTag := string(f.Tag)
			if gotTag != tt.wantTag {
				t.Fatalf("WebhookLog.%s tag = %q, want %q", tt.field, gotTag, tt.wantTag)
			}
		})
	}
}

// TestWebhookLogTableName verifies TableName returns the expected table name.
func TestWebhookLogTableName(t *testing.T) {
	want := "webhook_logs"
	got := WebhookLog{}.TableName()
	if got != want {
		t.Fatalf("WebhookLog.TableName() = %q, want %q", got, want)
	}
}

// TestWebhookLogFieldCount is a safety check: if a new field is added to
// WebhookLog, this test fails and reminds the developer to update the
// table-driven tests above.
func TestWebhookLogFieldCount(t *testing.T) {
	typ := reflect.TypeOf(WebhookLog{})
	if got := typ.NumField(); got != 9 {
		t.Fatalf("WebhookLog has %d fields, expected 9 — add new field tests to TestWebhookLogStructFields", got)
	}
}

// ensureDeletedAtCompiles is a compile-time check that gorm.io/gorm is importable.
// The DeletedAt field type gorm.DeletedAt is exercised in the struct field tests;
// this variable ensures the import is used.
var _ = gorm.DeletedAt{}
