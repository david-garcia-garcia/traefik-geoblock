package geoblock

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log/slog"
)

func TestLoadIPBlockHelper_MissingDir(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	helper, err := loadIPBlockHelper(nil, "/nonexistent/directory", logger)
	if err != nil {
		t.Fatalf("missing directory must not fail: %v", err)
	}
	found, _, _, err := helper.Contains(net.ParseIP("192.168.1.1"))
	if err != nil {
		t.Fatalf("Contains: %v", err)
	}
	if found {
		t.Fatalf("empty list must not match")
	}
}

func TestLoadIPBlockHelper_StaticAndDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tempDir := t.TempDir()
	writeIPBlockFile(t, filepath.Join(tempDir, "blocks.txt"), []string{"192.168.0.0/16"})

	helper, err := loadIPBlockHelper([]string{"10.0.0.0/8", "172.16.0.0/12"}, tempDir, logger)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	cases := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"8.8.8.8", false},
	}
	for _, tc := range cases {
		found, _, _, err := helper.Contains(net.ParseIP(tc.ip))
		if err != nil {
			t.Fatalf("%s: %v", tc.ip, err)
		}
		if found != tc.want {
			t.Fatalf("%s: found %v want %v", tc.ip, found, tc.want)
		}
	}
}

func TestLoadIPBlockHelper_InvalidLines(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tempDir := t.TempDir()
	content := `# Valid block
192.168.0.0/16
# Invalid blocks below
invalid-cidr
192.168.1.0/33
not.an.ip/24
# Another valid block
10.0.0.0/8
`
	if err := os.WriteFile(filepath.Join(tempDir, "invalid.txt"), []byte(content), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	helper, err := loadIPBlockHelper(nil, tempDir, logger)
	if err != nil {
		t.Fatalf("invalid lines must not fail load: %v", err)
	}
	found, _, _, err := helper.Contains(net.ParseIP("192.168.1.1"))
	if err != nil || !found {
		t.Fatalf("valid /16: found %v err %v", found, err)
	}
	found, _, _, err = helper.Contains(net.ParseIP("10.0.0.1"))
	if err != nil || !found {
		t.Fatalf("valid /8: found %v err %v", found, err)
	}
}

func TestLoadIPBlockHelper_InvalidStaticCIDR(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	_, err := loadIPBlockHelper([]string{"not-a-cidr"}, "", logger)
	if err == nil {
		t.Fatal("invalid static CIDR must fail the load")
	}
}

func TestLoadIPBlockHelper_SkipsNonTxt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tempDir := t.TempDir()
	writeIPBlockFile(t, filepath.Join(tempDir, "blocks.list"), []string{"10.0.0.0/8"})
	helper, err := loadIPBlockHelper(nil, tempDir, logger)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	found, _, _, err := helper.Contains(net.ParseIP("10.0.0.1"))
	if err != nil {
		t.Fatalf("Contains: %v", err)
	}
	if found {
		t.Fatal("non-.txt file must not load")
	}
}

func TestLoadIPBlockHelper_FamilyIsolation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	helper, err := loadIPBlockHelper([]string{"0.0.0.0/0"}, "", logger)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	found, _, _, err := helper.Contains(net.ParseIP("2001:db8::1"))
	if err != nil {
		t.Fatalf("Contains: %v", err)
	}
	if found {
		t.Fatalf("IPv4 /0 must not match IPv6")
	}

	helper, err = loadIPBlockHelper([]string{"::/0"}, "", logger)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	found, _, _, err = helper.Contains(net.ParseIP("8.8.8.8"))
	if err != nil {
		t.Fatalf("Contains: %v", err)
	}
	if found {
		t.Fatalf("IPv6 /0 must not match IPv4")
	}
}

func writeIPBlockFile(t *testing.T, filename string, blocks []string) {
	t.Helper()
	if err := os.WriteFile(filename, []byte(strings.Join(blocks, "\n")+"\n"), 0600); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}
