package pdf

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"math/big"
)

// encryption is the PDF 2.0 standard security handler, revision 6:
// AES-256 with a random file key wrapped by the user and owner
// passwords (ISO 32000-2, 7.6.4.3).
type encryption struct {
	key            []byte
	u, ue, o, oe   []byte
	perms          []byte
	permissionBits int32
}

func newEncryption(password string) (*encryption, error) {
	pw := []byte(password)
	if len(pw) > 127 {
		pw = pw[:127]
	}
	// The owner password is random: nobody can strip the protection
	// without knowing the user password either.
	owner := make([]byte, 32)
	key := make([]byte, 32)
	salts := make([]byte, 32)
	for _, b := range [][]byte{owner, key, salts} {
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
	}
	e := &encryption{key: key, permissionBits: -4} // every permission
	uvs, uks, ovs, oks := salts[0:8], salts[8:16], salts[16:24], salts[24:32]

	e.u = append(append(hash2B(pw, uvs, nil), uvs...), uks...)
	ue, err := aesNoPad(hash2B(pw, uks, nil), key)
	if err != nil {
		return nil, err
	}
	e.ue = ue

	e.o = append(append(hash2B(owner, ovs, e.u), ovs...), oks...)
	oe, err := aesNoPad(hash2B(owner, oks, e.u), key)
	if err != nil {
		return nil, err
	}
	e.oe = oe

	perms := make([]byte, 16)
	binary.LittleEndian.PutUint32(perms, uint32(e.permissionBits))
	copy(perms[4:8], []byte{0xFF, 0xFF, 0xFF, 0xFF})
	copy(perms[8:12], "Tadb")
	if _, err := rand.Read(perms[12:16]); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	e.perms = make([]byte, 16)
	block.Encrypt(e.perms, perms) // a single ECB block
	return e, nil
}

func (e *encryption) dict() string {
	return fmt.Sprintf("<< /Filter /Standard /V 5 /R 6 /Length 256 "+
		"/CF << /StdCF << /Type /CryptFilter /CFM /AESV3 /AuthEvent /DocOpen /Length 32 >> >> "+
		"/StmF /StdCF /StrF /StdCF /P %d /O <%x> /U <%x> /OE <%x> /UE <%x> /Perms <%x> /EncryptMetadata true >>",
		e.permissionBits, e.o, e.u, e.oe, e.ue, e.perms)
}

// encrypt encrypts a string or stream: AES-256-CBC with a random IV
// in front and PKCS#7 padding.
func (e *encryption) encrypt(data []byte) []byte {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		panic(err) // the key is always 32 bytes
	}
	pad := 16 - len(data)%16
	buf := make([]byte, 16+len(data)+pad)
	iv := buf[:16]
	if _, err := rand.Read(iv); err != nil {
		panic(err)
	}
	copy(buf[16:], data)
	for i := 16 + len(data); i < len(buf); i++ {
		buf[i] = byte(pad)
	}
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(buf[16:], buf[16:])
	return buf
}

// aesNoPad encrypts with AES-256-CBC, a zero IV and no padding, as
// the handler wraps the file key.
func aesNoPad(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, make([]byte, 16)).CryptBlocks(out, data)
	return out, nil
}

// hash2B is the revision 6 password hash (algorithm 2.B).
func hash2B(pw, salt, udata []byte) []byte {
	h := sha256.New()
	h.Write(pw)
	h.Write(salt)
	h.Write(udata)
	k := h.Sum(nil)
	for round := 0; ; round++ {
		seq := make([]byte, 0, len(pw)+len(k)+len(udata))
		seq = append(seq, pw...)
		seq = append(seq, k...)
		seq = append(seq, udata...)
		k1 := make([]byte, 0, 64*len(seq))
		for i := 0; i < 64; i++ {
			k1 = append(k1, seq...)
		}
		block, _ := aes.NewCipher(k[:16])
		e := make([]byte, len(k1))
		cipher.NewCBCEncrypter(block, k[16:32]).CryptBlocks(e, k1)
		switch new(big.Int).Mod(new(big.Int).SetBytes(e[:16]), big.NewInt(3)).Int64() {
		case 0:
			s := sha256.Sum256(e)
			k = s[:]
		case 1:
			s := sha512.Sum384(e)
			k = s[:]
		default:
			s := sha512.Sum512(e)
			k = s[:]
		}
		if round >= 63 && int(e[len(e)-1]) <= round-31 {
			break
		}
	}
	return k[:32]
}
