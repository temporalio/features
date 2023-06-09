package encryption

// Adapted from samples-go/encryption/data_converter.go and crypto.go

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"io"
)

const (
	MetadataEncodingEncrypted = "binary/encrypted"
	MetadataEncryptionKeyID   = "encryption-key-id"
	TEST_KEY_STR              = "test-key-test-key-test-key-test!"
)

type DataConverter struct {
	converter.DataConverter
	options DataConverterOptions
}

type DataConverterOptions struct {
	KeyID string
}

// NewEncryptionDataConverter creates a new instance of EncryptionDataConverter wrapping a DataConverter
func NewEncryptionDataConverter(dataConverter converter.DataConverter, options DataConverterOptions) *DataConverter {
	codecs := []converter.PayloadCodec{
		&Codec{KeyID: options.KeyID},
	}

	return &DataConverter{
		DataConverter: converter.NewCodecDataConverter(dataConverter, codecs...),
		options:       options,
	}
}

// Codec implements PayloadCodec using AES Crypt.
type Codec struct {
	KeyID string
}

func (e *Codec) getKey(keyID string) (key []byte) {
	// For testing here we just hard code a key.
	return []byte(TEST_KEY_STR)
}

// Encode implements converter.PayloadCodec.Encode.
func (e *Codec) Encode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		origBytes, err := p.Marshal()
		if err != nil {
			return payloads, err
		}

		key := e.getKey(e.KeyID)

		b, err := encrypt(origBytes, key)
		if err != nil {
			return payloads, err
		}

		result[i] = &commonpb.Payload{
			Metadata: map[string][]byte{
				converter.MetadataEncoding: []byte(MetadataEncodingEncrypted),
				MetadataEncryptionKeyID:    []byte(e.KeyID),
			},
			Data: b,
		}
	}

	return result, nil
}

// Decode implements converter.PayloadCodec.Decode.
func (e *Codec) Decode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	result := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		// Only if it's encrypted
		if string(p.Metadata[converter.MetadataEncoding]) != MetadataEncodingEncrypted {
			result[i] = p
			continue
		}

		keyID, ok := p.Metadata[MetadataEncryptionKeyID]
		if !ok {
			return payloads, fmt.Errorf("no encryption key id")
		}

		key := e.getKey(string(keyID))

		b, err := decrypt(p.Data, key)
		if err != nil {
			return payloads, err
		}

		result[i] = &commonpb.Payload{}
		err = result[i].Unmarshal(b)
		if err != nil {
			return payloads, err
		}
	}

	return result, nil
}

func encrypt(plainData []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plainData, nil), nil
}

func decrypt(encryptedData []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short: %v", encryptedData)
	}

	nonce, encryptedData := encryptedData[:nonceSize], encryptedData[nonceSize:]
	return gcm.Open(nil, nonce, encryptedData, nil)
}
