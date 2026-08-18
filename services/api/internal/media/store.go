package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const URLPrefix = "/api/admin/media/"

type Config struct {
	LocalDirectory string
	R2AccessKeyID  string
	R2AccountID    string
	R2Bucket       string
	R2SecretKey    string
}

type Store struct {
	bucket         string
	client         *s3.Client
	localDirectory string
}

func New(ctx context.Context, config Config) (*Store, error) {
	store := &Store{localDirectory: config.LocalDirectory}
	if config.R2Bucket == "" {
		if err := os.MkdirAll(config.LocalDirectory, 0o755); err != nil {
			return nil, fmt.Errorf("create local media directory: %w", err)
		}
		return store, nil
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.R2AccessKeyID, config.R2SecretKey, "")),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("configure R2 client: %w", err)
	}
	store.bucket = config.R2Bucket
	store.client = s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", config.R2AccountID))
	})
	return store, nil
}

func URL(key string) string { return URLPrefix + key }

func KeyFromURL(value string) (string, bool) {
	key := strings.TrimPrefix(value, URLPrefix)
	return key, key != value && validKey(key)
}

func (store *Store) Remote() bool { return store.client != nil }

func (store *Store) Put(ctx context.Context, key, contentType string, data []byte) error {
	if !validKey(key) {
		return fmt.Errorf("invalid media key")
	}
	if store.client != nil {
		_, err := store.client.PutObject(ctx, &s3.PutObjectInput{
			Body:        bytes.NewReader(data),
			Bucket:      aws.String(store.bucket),
			ContentType: aws.String(contentType),
			Key:         aws.String(key),
		})
		return err
	}

	filename := filepath.Join(store.localDirectory, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".upload-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}

func (store *Store) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if !validKey(key) {
		return nil, "", fmt.Errorf("invalid media key")
	}
	if store.client != nil {
		object, err := store.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)})
		if err != nil {
			return nil, "", err
		}
		return object.Body, aws.ToString(object.ContentType), nil
	}
	file, err := os.Open(filepath.Join(store.localDirectory, filepath.FromSlash(key)))
	if err != nil {
		return nil, "", err
	}
	return file, mime.TypeByExtension(path.Ext(key)), nil
}

func (store *Store) Delete(ctx context.Context, key string) error {
	if !validKey(key) {
		return fmt.Errorf("invalid media key")
	}
	if store.client != nil {
		_, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(store.bucket), Key: aws.String(key)})
		return err
	}
	err := os.Remove(filepath.Join(store.localDirectory, filepath.FromSlash(key)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func validKey(key string) bool {
	return key != "" && key != "." && !strings.Contains(key, "\\") && !strings.HasPrefix(key, "/") && path.Clean(key) == key && !strings.HasPrefix(key, "../")
}
