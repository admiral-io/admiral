package state

import (
	"context"
	"io"

	"github.com/google/uuid"

	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service/authn"
	"go.admiral.io/admiral/internal/service/objectstorage"
)

type mockSessionProvider struct {
	verifyFunc func(ctx context.Context, credential string) (*authn.Claims, error)
}

func (m *mockSessionProvider) Verify(ctx context.Context, credential string) (*authn.Claims, error) {
	return m.verifyFunc(ctx, credential)
}

func (m *mockSessionProvider) RefreshSession(ctx context.Context, sessionToken string) error {
	return nil
}

type mockStateStore struct {
	getLatestFunc      func(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformState, error)
	createFunc         func(ctx context.Context, st *model.TerraformState) (*model.TerraformState, error)
	setStoragePathFunc func(ctx context.Context, id uuid.UUID, path string) error
	getLockFunc        func(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformStateLock, error)
	acquireLockFunc    func(ctx context.Context, lock *model.TerraformStateLock) (*model.TerraformStateLock, error)
	releaseLockFunc    func(ctx context.Context, componentID, environmentID uuid.UUID, lockID string) error
}

func (m *mockStateStore) GetLatest(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformState, error) {
	if m.getLatestFunc != nil {
		return m.getLatestFunc(ctx, componentID, environmentID)
	}
	return nil, nil
}

func (m *mockStateStore) Create(ctx context.Context, st *model.TerraformState) (*model.TerraformState, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, st)
	}
	return st, nil
}

func (m *mockStateStore) SetStoragePath(ctx context.Context, id uuid.UUID, path string) error {
	if m.setStoragePathFunc != nil {
		return m.setStoragePathFunc(ctx, id, path)
	}
	return nil
}

func (m *mockStateStore) GetLock(ctx context.Context, componentID, environmentID uuid.UUID) (*model.TerraformStateLock, error) {
	if m.getLockFunc != nil {
		return m.getLockFunc(ctx, componentID, environmentID)
	}
	return nil, nil
}

func (m *mockStateStore) AcquireLock(ctx context.Context, lock *model.TerraformStateLock) (*model.TerraformStateLock, error) {
	if m.acquireLockFunc != nil {
		return m.acquireLockFunc(ctx, lock)
	}
	return lock, nil
}

func (m *mockStateStore) ReleaseLock(ctx context.Context, componentID, environmentID uuid.UUID, lockID string) error {
	if m.releaseLockFunc != nil {
		return m.releaseLockFunc(ctx, componentID, environmentID, lockID)
	}
	return nil
}

type mockComponentStore struct {
	getFunc func(ctx context.Context, id uuid.UUID) (*model.Component, error)
}

func (m *mockComponentStore) Get(ctx context.Context, id uuid.UUID) (*model.Component, error) {
	return m.getFunc(ctx, id)
}

type mockEnvironmentStore struct {
	getFunc func(ctx context.Context, id uuid.UUID) (*model.Environment, error)
}

func (m *mockEnvironmentStore) Get(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	return m.getFunc(ctx, id)
}

type mockObjStore struct {
	getObjectFunc func(ctx context.Context, bucket, path string) ([]byte, error)
	putObjectFunc func(ctx context.Context, bucket, path string, content []byte) error
}

func (m *mockObjStore) GetObject(ctx context.Context, bucket, path string) ([]byte, error) {
	if m.getObjectFunc != nil {
		return m.getObjectFunc(ctx, bucket, path)
	}
	return nil, nil
}

func (m *mockObjStore) PutObject(ctx context.Context, bucket, path string, content []byte) error {
	if m.putObjectFunc != nil {
		return m.putObjectFunc(ctx, bucket, path, content)
	}
	return nil
}

func (m *mockObjStore) ListObjects(ctx context.Context, bucket, prefix string) ([]objectstorage.Object, error) {
	return nil, nil
}

func (m *mockObjStore) DeleteObject(ctx context.Context, bucket, path string) error {
	return nil
}

func (m *mockObjStore) Close() error {
	return nil
}

var _ io.Closer = (*mockObjStore)(nil)
