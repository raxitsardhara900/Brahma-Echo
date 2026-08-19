package state

import (
	"os"
	"strings"
	"testing"
)

// The .json/.json.enc extension records whether Save encrypted the payload.
// Load must trust that rather than the caller's key, or a plaintext state file
// becomes unreadable the moment an encryption key happens to be configured.
func TestLoadPlaintextStateWithKeyConfigured(t *testing.T) {
	dir := t.TempDir()
	saved, err := Save(dir, &StateFile{Name: "plain", Origins: []string{"https://example.com"}}, "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.HasSuffix(saved, ".json") || strings.HasSuffix(saved, ".json.enc") {
		t.Fatalf("expected a plaintext .json file, got %s", saved)
	}

	sf, err := Load(saved, "a-key-configured-for-other-reasons")
	if err != nil {
		t.Fatalf("Load(plaintext file, key set) = %v, want success", err)
	}
	if sf.Name != "plain" {
		t.Errorf("Name = %q, want plain", sf.Name)
	}
}

func TestLoadEncryptedStateRoundTrips(t *testing.T) {
	dir := t.TempDir()
	const key = "correct-horse-battery-staple"
	saved, err := Save(dir, &StateFile{Name: "secret", Origins: []string{"https://example.com"}}, key)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.HasSuffix(saved, ".json.enc") {
		t.Fatalf("expected an encrypted .json.enc file, got %s", saved)
	}

	sf, err := Load(saved, key)
	if err != nil {
		t.Fatalf("Load(encrypted, correct key) = %v, want success", err)
	}
	if sf.Name != "secret" || !sf.Encrypted {
		t.Errorf("got name=%q encrypted=%v, want secret/true", sf.Name, sf.Encrypted)
	}

	if _, err := Load(saved, "wrong-key"); err == nil {
		t.Error("Load(encrypted, wrong key) succeeded, want failure")
	}
}

// Reading an encrypted file with no key should say so rather than surfacing a
// JSON syntax error about binary bytes.
func TestLoadEncryptedStateWithoutKeyExplainsItself(t *testing.T) {
	dir := t.TempDir()
	saved, err := Save(dir, &StateFile{Name: "secret"}, "a-key")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err = Load(saved, "")
	if err == nil {
		t.Fatal("Load(encrypted, no key) succeeded, want failure")
	}
	if !strings.Contains(err.Error(), "encrypted") {
		t.Errorf("error = %v, want it to mention the file is encrypted", err)
	}
}

// Save writes through a temp file; the temp must never survive, and it must not
// be picked up as a state file by List/Clean.
func TestSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, &StateFile{Name: "session"}, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := Save(dir, &StateFile{Name: "session"}, ""); err != nil {
		t.Fatalf("Save (overwrite): %v", err)
	}

	entries, err := os.ReadDir(SessionsDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file %q left behind after save", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("sessions dir has %d entries, want 1", len(entries))
	}

	list, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "session" {
		t.Errorf("List() = %+v, want a single \"session\" entry", list)
	}
}
