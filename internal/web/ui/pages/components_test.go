package ui

import (
	"context"
	viewmodel "gostarter/internal/web/ui"
	"strings"
	"testing"
)

func TestAdminPageKeepsLayoutAndSessionBoundary(t *testing.T) {
	var login strings.Builder
	if err := AdminPage(AdminPageModel{}).Render(context.Background(), &login); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(login.String(), `id="settingsForm"`) || !strings.Contains(login.String(), `hx-post="/ui/admin/login"`) {
		t.Fatal("login leaked dashboard or lacks HTMX")
	}
	var dashboard strings.Builder
	model := AdminPageModel{User: &viewmodel.User{Username: "<script>alert(1)</script>"}, Settings: viewmodel.Settings{DefaultProtocol: "grpc", UpstreamRepo: "owner/repo"}}
	if err := AdminPage(model).Render(context.Background(), &dashboard); err != nil {
		t.Fatal(err)
	}
	html := dashboard.String()
	for _, value := range []string{"max-w-7xl", "lg:grid-cols-2", `hx-post="/ui/admin/settings"`, `hx-get="/ui/admin/status"`, `value="grpc" selected`, `value="owner/repo"`} {
		if !strings.Contains(html, value) {
			t.Fatalf("missing %s", value)
		}
	}
	if strings.Contains(html, model.User.Username) {
		t.Fatal("username was not escaped")
	}
	if strings.Contains(html, `value="rest"`) {
		t.Fatal("REST-only returned")
	}
}
