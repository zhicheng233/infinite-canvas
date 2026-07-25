package service

import "testing"

func TestValidateWebhookDeliveryResponse(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		body     string
		wantErr  bool
	}{
		{name: "telegram success", platform: "telegram", body: `{"ok":true}`},
		{name: "telegram rejected", platform: "telegram", body: `{"ok":false,"description":"Bad Request"}`, wantErr: true},
		{name: "feishu success", platform: "feishu", body: `{"code":0,"msg":"success"}`},
		{name: "feishu legacy success", platform: "feishu", body: `{"StatusCode":0,"StatusMessage":"success"}`},
		{name: "feishu rejected", platform: "feishu", body: `{"code":19001,"msg":"invalid webhook"}`, wantErr: true},
		{name: "dingtalk success", platform: "dtalk", body: `{"errcode":0,"errmsg":"ok"}`},
		{name: "dingtalk rejected", platform: "dtalk", body: `{"errcode":310000,"errmsg":"invalid token"}`, wantErr: true},
		{name: "wecom success", platform: "wecom", body: `{"errcode":0,"errmsg":"ok"}`},
		{name: "wecom rejected", platform: "wecom", body: `{"errcode":93000,"errmsg":"invalid webhook"}`, wantErr: true},
		{name: "malformed response", platform: "telegram", body: `not-json`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebhookDeliveryResponse(tt.platform, []byte(tt.body))
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
