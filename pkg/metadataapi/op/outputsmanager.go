package op

import (
	"bytes"
	"context"
	"crypto/sha1"
	"io"

	"github.com/puppetlabs/horsehead/encoding/transfer"
	"github.com/puppetlabs/nebula-sdk/pkg/outputs"
	"github.com/puppetlabs/nebula-tasks/pkg/errors"
)

type OutputsManager interface {
	Get(ctx context.Context, taskName, key string) (*outputs.Output, errors.Error)
	Put(ctx context.Context, taskHash [sha1.Size]byte, key string, value io.Reader) errors.Error
}

type EncodeDecodingOutputsManager struct {
	delegate OutputsManager
}

func (m EncodeDecodingOutputsManager) Get(ctx context.Context, taskName, key string) (*outputs.Output, errors.Error) {
	out, err := m.delegate.Get(ctx, taskName, key)
	if err != nil {
		return nil, err
	}

	decoded, derr := transfer.DecodeFromTransfer(out.Value)
	if derr != nil {
		return nil, errors.NewOutputsValueDecodingError().WithCause(derr)
	}

	out.Value = string(decoded)

	return out, nil
}

func (m EncodeDecodingOutputsManager) Put(ctx context.Context, taskHash [sha1.Size]byte, key string, value io.Reader) errors.Error {
	buf := &bytes.Buffer{}

	_, err := buf.ReadFrom(value)
	if err != nil {
		return errors.NewOutputsValueReadError().WithCause(err)
	}

	encoded, err := transfer.EncodeForTransfer(buf.Bytes())
	if err != nil {
		return errors.NewOutputsValueEncodingError().WithCause(err)
	}

	buf = bytes.NewBufferString(encoded)

	return m.delegate.Put(ctx, taskHash, key, buf)
}

func NewEncodeDecodingOutputsManager(om OutputsManager) *EncodeDecodingOutputsManager {
	return &EncodeDecodingOutputsManager{delegate: om}
}
