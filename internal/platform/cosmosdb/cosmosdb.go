package cosmosdb

import (
	"errors"
	"fmt"

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

func StatusCode(err error) int {
	var re *azcore.ResponseError
	if errors.As(err, &re) {
		return re.StatusCode
	}
	return 0
}
