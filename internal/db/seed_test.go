package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inboxes-cig/bastion/internal/auth"
)

func TestReadInitialAdmin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-admin")
	if err := os.WriteFile(path, []byte("# comment\nemail=admin@example.com\npassword=Secret=123!\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok := readInitialAdmin(path)
	if !ok {
		t.Fatal("expected initial admin file to be recognized")
	}
	if got.Email != "admin@example.com" {
		t.Fatalf("email = %q", got.Email)
	}
	if got.Password != "Secret=123!" {
		t.Fatalf("password = %q", got.Password)
	}
}

func TestReadInitialAdminMissing(t *testing.T) {
	if _, ok := readInitialAdmin(filepath.Join(t.TempDir(), "nope")); ok {
		t.Fatal("missing file must not be recognized")
	}
}

// TestSeedInitialAdmin is an integration test. It only runs when
// TEST_DATABASE_URL points to a disposable PostgreSQL database.
func TestSeedInitialAdmin(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	d, err := Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error; err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(d); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "initial-admin")
	if err := os.WriteFile(path, []byte("email=initial@example.com\npassword=Initial123!\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := InitialAdminPath
	InitialAdminPath = path
	defer func() { InitialAdminPath = old }()

	res, err := Seed(d)
	if err != nil {
		t.Fatal(err)
	}
	if res.Email != "initial@example.com" || res.Password != "Initial123!" {
		t.Fatalf("result = %+v", res)
	}
	var u User
	if err := d.Where("email = ?", "initial@example.com").First(&u).Error; err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword(u.PasswordHash, "Initial123!") {
		t.Fatal("password hash does not match")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("initial-admin file should be deleted")
	}

	// A second run without the file must not create a legacy admin.
	res2, err := Seed(d)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Password != "" {
		t.Fatalf("second seed created another admin: %+v", res2)
	}
	var count int64
	d.Model(&User{}).Count(&count)
	if count != 1 {
		t.Fatalf("user count = %d, want 1", count)
	}
}
