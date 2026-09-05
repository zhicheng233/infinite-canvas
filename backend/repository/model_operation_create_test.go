package repository

import (
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"infinite-canvas-server/model"
)

func TestModelOperationCreatePreservesEnabledDryRun(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(localhost:3306)/unused?parseTime=true", SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"single_disabled", "single_enabled", "batch_mixed"} {
		t.Run(name, func(t *testing.T) {
			want := []bool{false}
			if name == "single_enabled" {
				want = []bool{true}
			} else if name == "batch_mixed" {
				want = []bool{false, true}
			}
			items := make([]model.ChannelModelOperation, len(want))
			for i, enabled := range want {
				items[i] = model.ChannelModelOperation{ChannelModelID: uint(i + 1), Capability: "image", Operation: "generate", Enabled: enabled, ProtocolMode: model.ProtocolModeInherit, ConfigJSON: "{}", ConfigVersion: 1}
			}
			var result *gorm.DB
			if len(items) == 1 {
				result = db.Create(&items[0])
			} else {
				result = db.Create(&items)
			}
			if result.Error != nil {
				t.Fatal(result.Error)
			}
			values, ok := result.Statement.Clauses["VALUES"].Expression.(clause.Values)
			if !ok || len(values.Values) != len(want) {
				t.Fatalf("unexpected insert values: %#v", values)
			}
			enabledColumn := -1
			for i, column := range values.Columns {
				if column.Name == "enabled" {
					enabledColumn = i
				}
			}
			if enabledColumn < 0 {
				t.Fatal("INSERT must explicitly include enabled")
			}
			for i, enabled := range want {
				if got := result.Statement.Vars[i*len(values.Columns)+enabledColumn]; got != enabled {
					t.Errorf("row %d SQL enabled=%v, want %v", i, got, enabled)
				}
				if items[i].Enabled != enabled {
					t.Errorf("row %d input enabled mutated to %v", i, items[i].Enabled)
				}
				if items[i].CreatedAt.IsZero() || items[i].UpdatedAt.IsZero() {
					t.Errorf("row %d missing automatic timestamps", i)
				}
			}
		})
	}
}
