package upstream

import (
	"testing"
)

type settingsRecorder struct {
	calls int
	saved AdminSettings
}

func (r *settingsRecorder) SaveSettings(s AdminSettings) error {
	r.calls++
	r.saved = s
	return nil
}

func TestSettingsValidationPrecedesPersistence(t *testing.T) {
	for _, input := range []AdminSettings{
		{DefaultProtocol: "rest"},
		{DefaultProtocol: "dual", DefaultGroup: "invalid module", DefaultService: "service", DefaultGoVersion: "1.27.1"},
	} {
		repository := &settingsRecorder{}
		if err := NewSettings(repository).SaveSettings(&input); err == nil {
			t.Fatal("invalid settings accepted")
		}
		if repository.calls != 0 {
			t.Fatal("invalid settings persisted")
		}
	}
}
func TestSettingsNormalizesBeforePersistence(t *testing.T) {
	repository := &settingsRecorder{}
	input := AdminSettings{DefaultProtocol: "dual", DefaultGroup: "github.com/example/", DefaultService: "service", DefaultGoVersion: "1.27.1"}
	if err := NewSettings(repository).SaveSettings(&input); err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || repository.saved.DefaultProtocol != "dual" || repository.saved.DefaultGoVersion != "1.27.1" {
		t.Fatalf("unexpected settings: %+v", repository)
	}
}
