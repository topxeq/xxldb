// Package crypto provides TXDEF encryption for xxldb
package crypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
)

const (
	// Magic bytes for encrypted files
	MagicTXDEF = "//TXDEF#"
	// Header length
	HeaderLen = 8
	// Buffer size for stream operations
	BufferSize = 4096
)

var (
	ErrInvalidMagic     = errors.New("invalid encryption magic bytes")
	ErrDecryptionFailed = errors.New("decryption failed")
)

// EncryptData encrypts data using TXDEF algorithm
func EncryptData(srcData []byte, code string) []byte {
	if srcData == nil {
		return nil
	}

	dataLen := len(srcData)
	if dataLen < 1 {
		return srcData
	}

	if code == "" {
		code = "xxldb"
	}

	codeBytes := []byte(code)
	codeLen := len(codeBytes)

	sum := sumBytes(codeBytes)
	addLen := int((sum % 5) + 2)
	encIndex := int(sum) % addLen

	head := []byte(MagicTXDEF)
	headLen := len(head)

	buf := make([]byte, dataLen+addLen+headLen)

	// Write header
	for i := 0; i < headLen; i++ {
		buf[i] = head[i]
	}

	// Generate random bytes
	randomBytes := make([]byte, addLen)
	rand.Read(randomBytes)

	var idxByte byte
	for i := 0; i < addLen; i++ {
		buf[i+headLen] = randomBytes[i]
		if i == encIndex {
			idxByte = randomBytes[i]
		}
	}

	// Encrypt data
	for i := 0; i < dataLen; i++ {
		buf[addLen+i+headLen] = srcData[i] + codeBytes[i%codeLen] + byte(i+1) + idxByte
	}

	return buf
}

// DecryptData decrypts data using TXDEF algorithm
func DecryptData(srcData []byte, code string) ([]byte, error) {
	if srcData == nil {
		return nil, nil
	}

	if code == "" {
		code = "xxldb"
	}

	codeBytes := []byte(code)
	codeLen := len(codeBytes)

	sum := sumBytes(codeBytes)
	addLen := int((sum % 5) + 2)
	encIndex := int(sum) % addLen

	// Check and strip header
	if bytes.HasPrefix(srcData, []byte(MagicTXDEF)) {
		srcData = srcData[HeaderLen:]
	} else {
		return nil, ErrInvalidMagic
	}

	dataLen := len(srcData)
	if dataLen < addLen {
		return nil, ErrDecryptionFailed
	}

	dataLen -= addLen

	buf := make([]byte, dataLen)

	// Decrypt data
	for i := 0; i < dataLen; i++ {
		buf[i] = srcData[addLen+i] - codeBytes[i%codeLen] - byte(i+1) - srcData[encIndex]
	}

	return buf, nil
}

// EncryptStream encrypts data from reader to writer using TXDEF algorithm
func EncryptStream(reader io.Reader, writer io.Writer, code string) error {
	if reader == nil || writer == nil {
		return errors.New("nil reader or writer")
	}

	if code == "" {
		code = "xxldb"
	}

	codeBytes := []byte(code)
	codeLen := len(codeBytes)

	sum := sumBytes(codeBytes)
	addLen := int((sum % 5) + 2)
	encIndex := int(sum) % addLen

	// Generate random bytes
	randomBytes := make([]byte, addLen)
	rand.Read(randomBytes)

	var idxByte byte
	for i := 0; i < addLen; i++ {
		if i == encIndex {
			idxByte = randomBytes[i]
		}
	}

	// Write header
	_, err := writer.Write([]byte(MagicTXDEF))
	if err != nil {
		return err
	}

	// Write random bytes
	_, err = writer.Write(randomBytes)
	if err != nil {
		return err
	}

	buf := make([]byte, BufferSize)
	i := 0

	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if n > 0 {
			// Encrypt buffer
			encBuf := make([]byte, n)
			for j := 0; j < n; j++ {
				encBuf[j] = buf[j] + codeBytes[i%codeLen] + byte(i+1) + idxByte
				i++
			}

			_, err := writer.Write(encBuf)
			if err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		}
	}

	return nil
}

// DecryptStream decrypts data from reader to writer using TXDEF algorithm
func DecryptStream(reader io.Reader, writer io.Writer, code string) error {
	if reader == nil || writer == nil {
		return errors.New("nil reader or writer")
	}

	if code == "" {
		code = "xxldb"
	}

	codeBytes := []byte(code)
	codeLen := len(codeBytes)

	sum := sumBytes(codeBytes)
	addLen := int((sum % 5) + 2)
	encIndex := int(sum) % addLen

	// Read header
	header := make([]byte, HeaderLen)
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return err
	}

	if string(header) != MagicTXDEF {
		return ErrInvalidMagic
	}

	// Read random bytes
	randomBytes := make([]byte, addLen)
	_, err = io.ReadFull(reader, randomBytes)
	if err != nil {
		return err
	}

	idxByte := randomBytes[encIndex]

	buf := make([]byte, BufferSize)
	i := 0

	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if n > 0 {
			// Decrypt buffer
			decBuf := make([]byte, n)
			for j := 0; j < n; j++ {
				decBuf[j] = buf[j] - codeBytes[i%codeLen] - byte(i+1) - idxByte
				i++
			}

			_, err := writer.Write(decBuf)
			if err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		}
	}

	return nil
}

// IsEncrypted checks if data is encrypted with TXDEF
func IsEncrypted(data []byte) bool {
	return bytes.HasPrefix(data, []byte(MagicTXDEF))
}

// IsEncryptedFile checks if data is encrypted with TXDEF (alias for IsEncrypted)
func IsEncryptedFile(data []byte) bool {
	return IsEncrypted(data)
}

// sumBytes calculates sum of bytes
func sumBytes(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
	}
	return sum
}

// Encryptor holds encryption state for reuse
type Encryptor struct {
	code      string
	codeBytes []byte
	codeLen   int
	addLen    int
	encIndex  int
}

// NewEncryptor creates a new encryptor with the given password
func NewEncryptor(password string, salt []byte) (*Encryptor, error) {
	// For TXDEF, we use password directly, salt is ignored for compatibility
	if password == "" {
		password = "xxldb"
	}

	codeBytes := []byte(password)
	sum := sumBytes(codeBytes)
	addLen := int((sum % 5) + 2)
	encIndex := int(sum) % addLen

	return &Encryptor{
		code:      password,
		codeBytes: codeBytes,
		codeLen:   len(codeBytes),
		addLen:    addLen,
		encIndex:  encIndex,
	}, nil
}

// Encrypt encrypts data
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	return EncryptData(plaintext, e.code), nil
}

// Decrypt decrypts data
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	return DecryptData(ciphertext, e.code)
}

// EncryptFile encrypts data with header
func (e *Encryptor) EncryptFile(plaintext []byte) ([]byte, error) {
	return EncryptData(plaintext, e.code), nil
}

// DecryptFile decrypts data with header
func (e *Encryptor) DecryptFile(data []byte) ([]byte, error) {
	return DecryptData(data, e.code)
}

// GetSalt returns empty salt for compatibility
func (e *Encryptor) GetSalt() []byte {
	return nil
}

// GenerateSalt generates a random salt (for compatibility)
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	return salt, err
}

// ReadHeader reads the encryption header
func ReadHeader(data []byte) (magic string, err error) {
	if len(data) < HeaderLen {
		return "", errors.New("data too short for header")
	}
	return string(data[:HeaderLen]), nil
}
