package application

import (
	"context"
	"sync"
	"time"

	"malus-be/internal/kernel"
)

type fakeBlobs struct {
	mu      sync.Mutex
	objects map[string][]byte
	deleted []string
}

func newFakeBlobs() *fakeBlobs {
	return &fakeBlobs{objects: make(map[string][]byte)}
}

func (f *fakeBlobs) put(name string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[name] = data
}

func (f *fakeBlobs) UploadURL(_ context.Context, name, _ string, _ time.Duration) (string, error) {
	return "https://blob.test/upload/" + name, nil
}

func (f *fakeBlobs) DownloadURL(_ context.Context, name, _, _ string, _ time.Duration) (string, error) {
	return "https://blob.test/download/" + name, nil
}

func (f *fakeBlobs) Size(_ context.Context, name string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[name]
	if !ok {
		return 0, kernel.NotFound("blob %s", name)
	}
	return int64(len(data)), nil
}

func (f *fakeBlobs) ReadHead(_ context.Context, name string, n int) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data := f.objects[name]
	if len(data) > n {
		data = data[:n]
	}
	return data, nil
}

func (f *fakeBlobs) Delete(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, name)
	f.deleted = append(f.deleted, name)
	return nil
}
