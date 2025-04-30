package utils

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	storage "github.com/utopiosphe/titan-storage-sdk"
)

var (
	apiKey     = os.Getenv("API_KEY")
	locatorURL = os.Getenv("LOCATOR_URL")
	token      = os.Getenv("TOKEN")
)

func TestBackoffRetryUploadAsset(t *testing.T) {

	s, err := storage.Initialize(&storage.Config{TitanURL: locatorURL, APIKey: apiKey})
	if err != nil {
		t.Fatal("NewStorage error ", err)
	}

	requestTimeout := 2 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	backoff := NewBackoffCall(s.UploadAsset, WithMaxAttempts(5), WithInitialBackoff(defaultInitialBackoff))

	progress := func(doneSize int64, totalSize int64) {
		t.Logf("upload %d of %d", doneSize, totalSize)
	}

	res, err := backoff.Start(ctx, ctx, "backoff.go", nil, progress)
	if err != nil {
		t.Fatal("BackoffCall error ", err)
	}

	var cid cid.Cid
	err = res.ToDest(&cid)
	if err != nil {
		t.Fatal("ToDest error ", err)
	}

	fmt.Println(cid)
}

func TestBackoffRetryUploadStreamV2(t *testing.T) {

	s, err := storage.Initialize(&storage.Config{TitanURL: locatorURL, APIKey: apiKey})
	if err != nil {
		t.Fatal("NewStorage error ", err)
	}

	requestTimeout := 2 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	backoff := NewBackoffCall(s.UploadStreamV2, WithMaxAttempts(5), WithInitialBackoff(defaultInitialBackoff))

	progress := func(doneSize int64, totalSize int64) {
		t.Logf("upload %d of %d", doneSize, totalSize)
	}

	f, _ := os.Open("backoff.go")

	res, err := backoff.Start(ctx, ctx, f, f.Name(), progress)
	if err != nil {
		t.Fatal("BackoffCall error ", err)
	}

	var cid cid.Cid
	err = res.ToDest(&cid)
	if err != nil {
		t.Fatal("ToDest error ", err)
	}

	fmt.Println(cid)
}
