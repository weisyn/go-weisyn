package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	fileconfig "github.com/weisyn/v1/internal/config/storage/file"
	infralog "github.com/weisyn/v1/internal/core/infrastructure/log"
)

func TestFileStore_PathGuardsAndBlocksPrefix(t *testing.T) {
	ctx := context.Background()

	tmp, err := os.MkdirTemp("", "weisyn-filestore-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })

	filesRoot := filepath.Join(tmp, "files")

	cfg := fileconfig.NewFromOptions(&fileconfig.FileOptions{
		RootPath:                filesRoot,
		MaxFileSize:             1024, // MB
		DirectoryIndexEnabled:   false,
		FileVerificationEnabled: false,
		FilePermissions:         0600,
		DirectoryPermissions:    0700,
	})

	store := New(cfg, infralog.GetLogger())
	if store == nil {
		t.Fatalf("New store returned nil")
	}

	// blocks/ 前缀应可用（写入到 {instance_root}/blocks/...，而不是 {instance_root}/files/blocks/...）
	if err := store.MakeDir(ctx, "blocks/0000000000", true); err != nil {
		t.Fatalf("MakeDir(blocks): %v", err)
	}
	if err := store.Save(ctx, "blocks/0000000000/0000000001.bin", []byte("ok")); err != nil {
		t.Fatalf("Save(blocks): %v", err)
	}
	if b, err := store.Load(ctx, "blocks/0000000000/0000000001.bin"); err != nil || string(b) != "ok" {
		t.Fatalf("Load(blocks) got=%q err=%v", string(b), err)
	}

	// 禁止 ../ 越界
	if _, err := store.Load(ctx, "../blocks/0000000000/0000000001.bin"); err == nil {
		t.Fatalf("expected error for ../ traversal, got nil")
	}

	// 禁止绝对路径绕过
	if _, err := store.Load(ctx, filepath.Join(tmp, "blocks", "0000000000", "0000000001.bin")); err == nil {
		t.Fatalf("expected error for absolute path, got nil")
	}
}


