package connectors

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Backblaze/blazer/b2"
	"github.com/PlakarKorp/kloset/connectors/storage"
	"github.com/PlakarKorp/kloset/location"
	"github.com/PlakarKorp/kloset/objects"
)

type b2Store struct {
	bucketName string
	rootPath   string
	keyId      string
	appKey     string

	client *b2.Client
	bucket *b2.Bucket
}

func init() {
	storage.Register("b2", 0, NewStore)
}

func NewStore(ctx context.Context, proto string, config map[string]string) (storage.Store, error) {
	bucketName, prefix, keyId, appKey, err := parse(config, proto)

	if err != nil {
		return nil, err
	}
	return &b2Store{
		bucketName: bucketName,
		rootPath:   prefix,
		keyId:      keyId,
		appKey:     appKey,
	}, nil
}

func (e *b2Store) Root() string   { return e.rootPath }
func (e *b2Store) Origin() string { return e.bucketName } // Not the real endpoint of the bucket.
func (e *b2Store) Type() string   { return "b2" }

func (e *b2Store) Flags() location.Flags { return 0 }

func (e *b2Store) Ping(ctx context.Context) error {
	return nil
}

func (e *b2Store) Mode(context.Context) (storage.Mode, error) {
	// TODO Not based on keyId/app/Key rights.
	return storage.ModeRead | storage.ModeWrite, nil
}

func (e *b2Store) Create(ctx context.Context, config []byte) error {
	if err := e.connect(ctx); err != nil {
		return err
	}

	cfgPath := e.realpath("CONFIG")
	cfgObject := e.bucket.Object(cfgPath)
	if _, err := cfgObject.Attrs(ctx); err == nil {
		return fmt.Errorf("repository already exists")
	}

	writer := cfgObject.NewWriter(ctx)
	if _, err := writer.Write(config); err != nil {
		writer.Close()
		return err
	}

	return writer.Close()
}

func (e *b2Store) Open(ctx context.Context) ([]byte, error) {
	if err := e.connect(ctx); err != nil {
		return nil, err
	}

	cfgPath := e.realpath("CONFIG")
	cfgObject := e.bucket.Object(cfgPath)

	reader := cfgObject.NewReader(ctx)
	defer reader.Close()
	return io.ReadAll(reader)
}

func (e *b2Store) Close(ctx context.Context) error {
	e.client = nil
	e.bucket = nil
	return nil
}

func (e *b2Store) List(ctx context.Context, res storage.StorageResource) (ret []objects.MAC, err error) {
	prefix, err := res2prefix(res)
	if err != nil {
		return nil, err
	}

	prefix = e.realpath(prefix)
	l := len(prefix) + 4 // Correspond au préfixe + "/XX/" (les dossiers intermédiaires)

	iterator := e.bucket.List(ctx, b2.ListPrefix(prefix))
	for iterator.Next() {
		obj := iterator.Object()
		name := obj.Name()

		if len(name) <= l {
			continue // Ignore les objets mal formés ou trop courts
		}

		t, err := hex.DecodeString(name[l:])
		if err != nil {
			return nil, fmt.Errorf("decode %s key: %w", prefix, err)
		}
		if len(t) != 32 {
			return nil, fmt.Errorf("invalid %s name: %s", prefix, name)
		}

		ret = append(ret, objects.MAC(t))
	}

	return ret, nil
}

func (e *b2Store) Put(ctx context.Context, res storage.StorageResource, mac objects.MAC, rd io.Reader) (int64, error) {
	prefix, err := res2prefix(res)
	if err != nil {
		return -1, err
	}

	relPath := fmt.Sprintf("%s/%02x/%016x", prefix, mac[0], mac)
	objPath := e.realpath(relPath)

	obj := e.bucket.Object(objPath)
	writer := obj.NewWriter(ctx)

	n, err := io.Copy(writer, rd)
	if err != nil {
		writer.Close()
		return -1, fmt.Errorf("failed to write %s object: %w", res, err)
	}

	if err := writer.Close(); err != nil {
		return 0, fmt.Errorf("failed to close %s object: %w", res, err)
	}

	return n, nil
}

func (e *b2Store) Get(ctx context.Context, res storage.StorageResource, mac objects.MAC, rg *storage.Range) (io.ReadCloser, error) {
	prefix, err := res2prefix(res)
	if err != nil {
		return nil, err
	}

	relPath := fmt.Sprintf("%s/%02x/%016x", prefix, mac[0], mac)
	objPath := e.realpath(relPath)

	obj := e.bucket.Object(objPath)

	if rg != nil {
		return obj.NewRangeReader(ctx, int64(rg.Offset), int64(rg.Length)), nil
	}
	return obj.NewReader(ctx), nil
}

func (e *b2Store) Delete(ctx context.Context, res storage.StorageResource, mac objects.MAC) error {
	prefix, err := res2prefix(res)
	if err != nil {
		return err
	}

	relPath := fmt.Sprintf("%s/%02x/%016x", prefix, mac[0], mac)
	objPath := e.realpath(relPath)

	return e.bucket.Object(objPath).Delete(ctx)
}

func (e *b2Store) Size(context.Context) (int64, error) {
	// leave to plakar the job of figuring the actual size using
	// the states.  it's usually implemented only if there is an
	// easy way of getting the space used by the store, and only
	// by it.
	return -1, nil
}

func (e *b2Store) connect(ctx context.Context) error {
	if e.bucket != nil {
		return nil
	}

	client, err := b2.NewClient(ctx, e.keyId, e.appKey)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(ctx, e.bucketName)
	if err != nil {
		return err
	}

	e.client = client
	e.bucket = bucket
	return nil
}

func (g *b2Store) realpath(rel string) string {
	if g.rootPath == "" {
		return rel
	}
	return path.Join(g.rootPath, rel)
}

func parse(params map[string]string, proto string) (bucketName, prefix, keyId, appKey string, err error) {
	for k, v := range params {
		switch k {
		case "key_id":
			keyId = v

		case "app_key":
			appKey = v

		case "location":
			// b2://bucket-name/prefix
			bucketName, prefix, _ = strings.Cut(strings.TrimPrefix(v, proto+"://"), "/")
			prefix = strings.Trim(prefix, "/")

		default:
			return "", "", "", "", fmt.Errorf("unknown option: %s", k)
		}
	}

	if bucketName == "" {
		return "", "", "", "", fmt.Errorf("missing bucket name in location")
	}

	return
}

func res2prefix(res storage.StorageResource) (string, error) {
	switch res {
	case storage.StorageResourceState:
		return "states", nil
	case storage.StorageResourcePackfile:
		return "packfiles", nil
	case storage.StorageResourceLock:
		return "locks", nil
	default:
		return "", fmt.Errorf("%w on %s", errors.ErrUnsupported, res)
	}
}
