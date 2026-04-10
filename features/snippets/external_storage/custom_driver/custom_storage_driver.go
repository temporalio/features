package customdriver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/proto"
)

// @@@SNIPSTART go-custom-storage-driver
type LocalDiskStorageDriver struct {
	storeDir string
}

func NewLocalDiskStorageDriver(storeDir string) converter.StorageDriver {
	return &LocalDiskStorageDriver{storeDir: storeDir}
}

func (d *LocalDiskStorageDriver) Name() string {
	return "my-local-disk"
}

func (d *LocalDiskStorageDriver) Type() string {
	return "local-disk"
}

func (d *LocalDiskStorageDriver) Store(
	ctx converter.StorageDriverStoreContext,
	payloads []*commonpb.Payload,
) ([]converter.StorageDriverClaim, error) {
	dir := d.storeDir
	if info, ok := ctx.Target.(converter.StorageDriverWorkflowInfo); ok && info.WorkflowID != "" {
		dir = filepath.Join(d.storeDir, info.Namespace, info.WorkflowID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}

	claims := make([]converter.StorageDriverClaim, len(payloads))
	for i, payload := range payloads {
		key := uuid.NewString() + ".bin"
		filePath := filepath.Join(dir, key)

		data, err := proto.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		if err := os.WriteFile(filePath, data, 0o644); err != nil {
			return nil, fmt.Errorf("write payload: %w", err)
		}

		claims[i] = converter.StorageDriverClaim{
			ClaimData: map[string]string{"path": filePath},
		}
	}
	return claims, nil
}

func (d *LocalDiskStorageDriver) Retrieve(
	ctx converter.StorageDriverRetrieveContext,
	claims []converter.StorageDriverClaim,
) ([]*commonpb.Payload, error) {
	payloads := make([]*commonpb.Payload, len(claims))
	for i, claim := range claims {
		filePath := claim.ClaimData["path"]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read payload: %w", err)
		}
		payload := &commonpb.Payload{}
		if err := proto.Unmarshal(data, payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
		payloads[i] = payload
	}
	return payloads, nil
}

// @@@SNIPEND
