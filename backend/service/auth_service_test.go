package service

import "testing"

func TestShouldBootstrapInitialAdmin(t *testing.T) {
	tests := []struct {
		name      string
		userCount int64
		username  string
		password  string
		want      bool
		wantErr   bool
	}{
		{name: "skip when users already exist", userCount: 1, username: "admin", password: "Admin1234", want: false, wantErr: false},
		{name: "skip when init admin not configured", userCount: 0, username: "", password: "", want: false, wantErr: false},
		{name: "error when username missing", userCount: 0, username: "", password: "Admin1234", want: false, wantErr: true},
		{name: "error when password missing", userCount: 0, username: "admin", password: "", want: false, wantErr: true},
		{name: "error when password too weak", userCount: 0, username: "admin", password: "12345678", want: false, wantErr: true},
		{name: "create when empty database and config valid", userCount: 0, username: "admin", password: "Admin1234", want: true, wantErr: false},
	}

	for _, tt := range tests {
		got, err := shouldBootstrapInitialAdmin(tt.userCount, tt.username, tt.password)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%s: err = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
		if got != tt.want {
			t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}
