package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/duclamdev/application-chat/backend/internal/platform/storage"
)

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	if root == "" {
		root = "data/storage"
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("tạo thư mục gốc lưu trữ local: %w", err)
	}
	if err := chmodBestEffort(root, 0o700); err != nil {
		return nil, fmt.Errorf("không thể kiểm tra quyền ghi thư mục lưu trữ local: %w", err)
	}
	return &Store{root: root}, nil
}

func (s *Store) Put(ctx context.Context, input storage.PutObjectInput) (storage.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return storage.ObjectInfo{}, err
	}
	if input.Key == "" {
		return storage.ObjectInfo{}, errors.New("khóa đối tượng lưu trữ là bắt buộc")
	}

	path, err := s.path(input.Key)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("tạo thư mục đối tượng lưu trữ: %w", err)
	}
	if err := chmodBestEffort(filepath.Dir(path), 0o700); err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("không thể kiểm tra quyền ghi thư mục đối tượng: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("tạo đối tượng lưu trữ: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil && !isChmodUnsupported(err) {
		return storage.ObjectInfo{}, fmt.Errorf("không thể bảo vệ quyền đọc đối tượng lưu trữ: %w", err)
	}

	size, err := io.Copy(file, input.Body)
	if err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("ghi đối tượng lưu trữ: %w", err)
	}

	return storage.ObjectInfo{
		Key:         input.Key,
		ContentType: input.ContentType,
		Size:        size,
	}, nil
}

func (s *Store) Get(ctx context.Context, key string) (*storage.GetObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path, err := s.path(key)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("mở đối tượng lưu trữ: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("đọc thông tin đối tượng lưu trữ: %w", err)
	}

	return &storage.GetObjectOutput{
		Info: storage.ObjectInfo{
			Key:  key,
			Size: stat.Size(),
		},
		Body: file,
	}, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xóa đối tượng lưu trữ: %w", err)
	}
	return nil
}

func (s *Store) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	probe := filepath.Join(s.root, ".health")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("ghi file kiểm tra lưu trữ local: %w", err)
	}
	if err := os.Remove(probe); err != nil {
		return fmt.Errorf("xóa file kiểm tra lưu trữ local: %w", err)
	}
	return nil
}

func (s *Store) path(key string) (string, error) {
	key = filepath.Clean(strings.TrimPrefix(key, "/"))
	if key == "." || strings.HasPrefix(key, "..") {
		return "", errors.New("khóa đối tượng lưu trữ không hợp lệ")
	}
	return filepath.Join(s.root, key), nil
}

func chmodBestEffort(path string, mode os.FileMode) error {
	if err := os.Chmod(path, mode); err != nil && !isChmodUnsupported(err) {
		return err
	}

	probe, err := os.CreateTemp(path, ".chmod-probe-*")
	if err != nil {
		return err
	}
	probePath := probe.Name()
	if closeErr := probe.Close(); closeErr != nil {
		_ = os.Remove(probePath)
		return closeErr
	}
	return os.Remove(probePath)
}

func isChmodUnsupported(err error) bool {
	return errors.Is(err, os.ErrPermission) ||
		errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.EACCES)
}
