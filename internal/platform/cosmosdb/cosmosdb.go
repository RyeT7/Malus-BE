package cosmosdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

func NewClient(endpoint, key string, allowKey bool) (*azcosmos.Client, error) {
	if key != "" {
		if !allowKey {
			return nil, errors.New("COSMOS_KEY is only allowed when APP_ENV=local; use a managed identity with Cosmos DB data-plane RBAC")
		}
		cred, err := azcosmos.NewKeyCredential(key)
		if err != nil {
			return nil, fmt.Errorf("cosmos key: %w", err)
		}
		return azcosmos.NewClientWithKey(endpoint, cred, nil)
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	return azcosmos.NewClient(endpoint, cred, nil)
}

func EnsureContainer(ctx context.Context, client *azcosmos.Client, database, container, partitionKeyPath string) error {
	_, err := client.CreateDatabase(ctx, azcosmos.DatabaseProperties{ID: database}, nil)
	if err != nil && StatusCode(err) != http.StatusConflict {
		return fmt.Errorf("create database %s: %w", database, err)
	}
	db, err := client.NewDatabase(database)
	if err != nil {
		return err
	}
	_, err = db.CreateContainer(ctx, azcosmos.ContainerProperties{
		ID: container,
		PartitionKeyDefinition: azcosmos.PartitionKeyDefinition{
			Paths: []string{partitionKeyPath},
		},
	}, nil)
	if err != nil && StatusCode(err) != http.StatusConflict {
		return fmt.Errorf("create container %s: %w", container, err)
	}
	return nil
}

func StatusCode(err error) int {
	var re *azcore.ResponseError
	if errors.As(err, &re) {
		return re.StatusCode
	}
	return 0
}
