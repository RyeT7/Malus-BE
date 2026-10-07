package blobstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"

	"malus-be/internal/kernel"
)

type Config struct {
	ConnectionString string
	AccountURL       string
	Container        string
	PublicEndpoint   string
}

type Store struct {
	service   *service.Client
	container *container.Client
	name      string
	sharedKey *service.SharedKeyCredential
	endpoint  string
	protocol  sas.Protocol
	now       func() time.Time
}

func New(cfg Config, allowKey bool) (*Store, error) {
	if cfg.Container == "" {
		return nil, errors.New("blob container name is required")
	}
	s := &Store{name: cfg.Container, now: time.Now}

	switch {
	case cfg.ConnectionString != "":
		if !allowKey {
			return nil, errors.New("BLOB_CONNECTION_STRING is only allowed when APP_ENV=local; use BLOB_ACCOUNT_URL with a managed identity")
		}
		parsed, err := parseConnectionString(cfg.ConnectionString)
		if err != nil {
			return nil, err
		}
		cred, err := service.NewSharedKeyCredential(parsed.accountName, parsed.accountKey)
		if err != nil {
			return nil, fmt.Errorf("blob shared key: %w", err)
		}
		client, err := service.NewClientWithSharedKeyCredential(parsed.blobEndpoint, cred, nil)
		if err != nil {
			return nil, err
		}
		s.service, s.sharedKey, s.protocol = client, cred, sas.ProtocolHTTPSandHTTP
	case cfg.AccountURL != "":
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("azure credential: %w", err)
		}
		client, err := service.NewClient(cfg.AccountURL, cred, nil)
		if err != nil {
			return nil, err
		}
		s.service, s.protocol = client, sas.ProtocolHTTPS
	default:
		return nil, errors.New("BLOB_ACCOUNT_URL (or BLOB_CONNECTION_STRING locally) is required")
	}

	s.container = s.service.NewContainerClient(cfg.Container)
	s.endpoint = strings.TrimRight(cfg.PublicEndpoint, "/")
	if s.endpoint == "" {
		s.endpoint = strings.TrimRight(s.service.URL(), "/")
	}
	return s, nil
}

func (s *Store) UploadURL(ctx context.Context, blobName, _ string, ttl time.Duration) (string, error) {
	return s.sign(ctx, blobName, ttl, sas.BlobSignatureValues{
		Permissions: (&sas.BlobPermissions{Create: true, Write: true}).String(),
	})
}

func (s *Store) DownloadURL(ctx context.Context, blobName, fileName, contentType string, ttl time.Duration) (string, error) {
	return s.sign(ctx, blobName, ttl, sas.BlobSignatureValues{
		Permissions:        (&sas.BlobPermissions{Read: true}).String(),
		ContentDisposition: contentDisposition(fileName),
		ContentType:        contentType,
	})
}

func (s *Store) Size(ctx context.Context, blobName string) (int64, error) {
	props, err := s.container.NewBlobClient(blobName).GetProperties(ctx, nil)
	if bloberror.HasCode(err, bloberror.BlobNotFound, bloberror.ContainerNotFound) {
		return 0, kernel.NotFound("blob %s", blobName)
	}
	if err != nil {
		return 0, fmt.Errorf("blob properties: %w", err)
	}
	if props.ContentLength == nil {
		return 0, nil
	}
	return *props.ContentLength, nil
}

func (s *Store) ReadHead(ctx context.Context, blobName string, n int) ([]byte, error) {
	resp, err := s.container.NewBlobClient(blobName).DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{Offset: 0, Count: int64(n)},
	})
	if err != nil {
		return nil, fmt.Errorf("read blob: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, int64(n)))
}

func (s *Store) Delete(ctx context.Context, blobName string) error {
	_, err := s.container.NewBlobClient(blobName).Delete(ctx, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.BlobNotFound) {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}

func (s *Store) EnsureContainer(ctx context.Context) error {
	_, err := s.container.Create(ctx, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("create container %s: %w", s.name, err)
	}
	return nil
}

func (s *Store) AllowBrowserUploads(ctx context.Context, origins []string) error {
	if len(origins) == 0 {
		return nil
	}
	_, err := s.service.SetProperties(ctx, &service.SetPropertiesOptions{
		CORS: []*service.CORSRule{{
			AllowedOrigins:  to.Ptr(strings.Join(origins, ",")),
			AllowedMethods:  to.Ptr("GET,HEAD,PUT,OPTIONS"),
			AllowedHeaders:  to.Ptr("*"),
			ExposedHeaders:  to.Ptr("*"),
			MaxAgeInSeconds: to.Ptr[int32](600),
		}},
	})
	if err != nil {
		return fmt.Errorf("set blob CORS: %w", err)
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	_, err := s.container.GetProperties(ctx, nil)
	return err
}

func (s *Store) sign(ctx context.Context, blobName string, ttl time.Duration, values sas.BlobSignatureValues) (string, error) {
	now := s.now().UTC()
	values.ContainerName = s.name
	values.BlobName = blobName
	values.Protocol = s.protocol
	values.StartTime = now.Add(-5 * time.Minute)
	values.ExpiryTime = now.Add(ttl)

	var (
		query sas.QueryParameters
		err   error
	)
	if s.sharedKey != nil {
		query, err = values.SignWithSharedKey(s.sharedKey)
	} else {
		var udc *service.UserDelegationCredential
		udc, err = s.service.GetUserDelegationCredential(ctx, service.KeyInfo{
			Start:  to.Ptr(values.StartTime.Format(sas.TimeFormat)),
			Expiry: to.Ptr(values.ExpiryTime.Format(sas.TimeFormat)),
		}, nil)
		if err == nil {
			query, err = values.SignWithUserDelegation(udc)
		}
	}
	if err != nil {
		return "", fmt.Errorf("sign blob link: %w", err)
	}
	return s.endpoint + "/" + url.PathEscape(s.name) + "/" + escapePath(blobName) + "?" + query.Encode(), nil
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func contentDisposition(fileName string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, fileName)
	return fmt.Sprintf(`inline; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(fileName))
}

type connectionString struct {
	accountName  string
	accountKey   string
	blobEndpoint string
}

func parseConnectionString(raw string) (connectionString, error) {
	values := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			values[strings.ToLower(key)] = value
		}
	}
	cs := connectionString{accountName: values["accountname"], accountKey: values["accountkey"], blobEndpoint: values["blobendpoint"]}
	if cs.accountName == "" || cs.accountKey == "" {
		return connectionString{}, errors.New("blob connection string needs AccountName and AccountKey")
	}
	if cs.blobEndpoint == "" {
		protocol, suffix := values["defaultendpointsprotocol"], values["endpointsuffix"]
		if protocol == "" {
			protocol = "https"
		}
		if suffix == "" {
			suffix = "core.windows.net"
		}
		cs.blobEndpoint = fmt.Sprintf("%s://%s.blob.%s", protocol, cs.accountName, suffix)
	}
	return cs, nil
}
