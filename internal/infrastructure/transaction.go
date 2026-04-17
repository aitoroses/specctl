package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
)

type PlannedWrite struct {
	Path string
	Data []byte
	Perm os.FileMode
}

type stagedWrite struct {
	PlannedWrite
	tempPath string
}

type backupFile struct {
	path    string
	data    []byte
	perm    os.FileMode
	existed bool
}

func CommitWritesAtomically(writes []PlannedWrite) error {
	_, err := CommitWritesAtomicallyWithRollback(writes)
	return err
}

type WriteRollback struct {
	committed []stagedWrite
	backups   []backupFile
}

func (r *WriteRollback) Rollback() error {
	if r == nil {
		return nil
	}
	rollbackWrites(r.committed, r.backups)
	return nil
}

func CommitWritesAtomicallyWithRollback(writes []PlannedWrite) (*WriteRollback, error) {
	if len(writes) == 0 {
		return &WriteRollback{}, nil
	}

	staged := make([]stagedWrite, 0, len(writes))
	for _, write := range writes {
		dir := filepath.Dir(write.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}

		tmp, err := os.CreateTemp(dir, TempSiblingPattern(write.Path))
		if err != nil {
			cleanupStaged(staged)
			return nil, fmt.Errorf("create temp file for %s: %w", write.Path, err)
		}

		if err := tmp.Chmod(write.Perm); err != nil {
			tmpPath := tmp.Name()
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			cleanupStaged(staged)
			return nil, fmt.Errorf("chmod temp file for %s: %w", write.Path, err)
		}
		if _, err := tmp.Write(write.Data); err != nil {
			tmpPath := tmp.Name()
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			cleanupStaged(staged)
			return nil, fmt.Errorf("write temp file for %s: %w", write.Path, err)
		}
		if err := tmp.Sync(); err != nil {
			tmpPath := tmp.Name()
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			cleanupStaged(staged)
			return nil, fmt.Errorf("sync temp file for %s: %w", write.Path, err)
		}

		tmpPath := tmp.Name()
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpPath)
			cleanupStaged(staged)
			return nil, fmt.Errorf("close temp file for %s: %w", write.Path, err)
		}

		staged = append(staged, stagedWrite{
			PlannedWrite: write,
			tempPath:     tmpPath,
		})
	}

	backups := make([]backupFile, 0, len(staged))
	committed := make([]stagedWrite, 0, len(staged))
	for _, write := range staged {
		backup, err := captureBackup(write.Path)
		if err != nil {
			cleanupStaged(staged)
			return nil, err
		}
		backups = append(backups, backup)

		if err := os.Rename(write.tempPath, write.Path); err != nil {
			rollbackWrites(committed, backups[:len(committed)])
			cleanupStaged(staged[len(committed):])
			return nil, fmt.Errorf("replace %s: %w", write.Path, err)
		}
		committed = append(committed, write)
	}

	return &WriteRollback{
		committed: committed,
		backups:   backups,
	}, nil
}

func cleanupStaged(staged []stagedWrite) {
	for _, write := range staged {
		if write.tempPath != "" {
			_ = os.Remove(write.tempPath)
		}
	}
}

func captureBackup(path string) (backupFile, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return backupFile{path: path, existed: false}, nil
	}
	if err != nil {
		return backupFile{}, fmt.Errorf("stat %s: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return backupFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	return backupFile{
		path:    path,
		data:    data,
		perm:    info.Mode(),
		existed: true,
	}, nil
}

func rollbackWrites(committed []stagedWrite, backups []backupFile) {
	for i := len(committed) - 1; i >= 0; i-- {
		backup := backups[i]
		if backup.existed {
			_ = WriteFileAtomically(backup.path, backup.data, backup.perm)
			continue
		}
		_ = os.Remove(committed[i].Path)
	}
}
