package rpc

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"testing"
)

//func TestComposeRM(t *testing.T) {
//	id, err := zkidentity.New("moo", "Alice McMoo")
//	if err != nil {
//		t.Fatal(err)
//	}
//	c := RMPrivateMessage{
//		Message: "Hello, world!",
//	}
//	msg, err := ComposeRM(id, c)
//	if err != nil {
//		t.Fatal(err)
//	}
//	//t.Logf("%s", spew.Sdump(msg))
//
//	m, p, err := DecomposeRM(id.Public, msg)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if m.Version != RMHeaderVersion {
//		t.Fatalf("version got %v want %v", m.Version, RMHeaderVersion)
//	}
//	if m.Command != RMCPrivateMessage {
//		t.Fatalf("command got %v want %v", m.Command, RMCPrivateMessage)
//	}
//	//t.Logf("%v", spew.Sdump(m))
//	//t.Logf("%v", spew.Sdump(p))
//
//	if !reflect.DeepEqual(c, p) {
//		t.Fatal("corrupt")
//	}
//}
//
//func random(b []byte) {
//	_, err := io.ReadFull(rand.Reader, b)
//	if err != nil {
//		panic(err)
//	}
//}
//
//func TestAllComposeRM(t *testing.T) {
//	id, err := zkidentity.New("moo", "Alice McMoo")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	tests := []struct {
//		i    interface{}
//		cmd  string
//		prep func() (interface{}, error)
//		want error
//	}{
//		{
//			RMPrivateMessage{
//				Message: "Hello, world!",
//			},
//			RMCPrivateMessage,
//			nil,
//			nil,
//		},
//		{
//			RMIdentityKX{},
//			RMCIdentityKX,
//			func() (interface{}, error) {
//				// Fill with random crap to verify decoder
//				fi, err := zkidentity.New("Moo McMoo", "moo")
//				if err != nil {
//					return nil, err
//				}
//				idkx := RMIdentityKX{
//					Identity: fi.Public,
//					KX: ratchet.KeyExchange{
//						Public: []byte("public"),
//					},
//				}
//				random(idkx.InitialRendezvous[:])
//				random(idkx.KX.Cipher[:])
//				return idkx, nil
//			},
//			nil,
//		},
//		{
//			RMKX{},
//			RMCKX,
//			func() (interface{}, error) {
//				// Fill with random crap to verify decoder
//				kx := RMKX{
//					KX: ratchet.KeyExchange{
//						Public: []byte("public"),
//					},
//				}
//				random(kx.KX.Cipher[:])
//				return kx, nil
//			},
//			nil,
//		},
//		{
//			RMInvite{},
//			RMCInvite,
//			func() (interface{}, error) {
//				// Fill with random crap to verify decoder
//				fi, err := zkidentity.New("Moo McMoo", "moo")
//				if err != nil {
//					return nil, err
//				}
//				invite := RMInvite{
//					Invitee: fi.Public,
//				}
//				return invite, nil
//			},
//			nil,
//		},
//		{
//			RMInviteReply{},
//			RMCInviteReply,
//			func() (interface{}, error) {
//				ir := RMInviteReply{
//					InviteBlob: make([]byte, 1500),
//				}
//				random(ir.For[:])
//				random(ir.InviteBlob)
//				return ir, nil
//			},
//			nil,
//		},
//	}
//
//	t.Logf("Running %d tests", len(tests))
//	for i, test := range tests {
//		if test.prep != nil {
//			munged, err := test.prep()
//			if err != nil {
//				t.Fatal(err)
//			}
//			test.i = munged
//			//t.Logf("%v", spew.Sdump(test.i))
//		}
//		msg, err := ComposeRM(id, test.i)
//		if err != nil {
//			t.Fatal(err)
//		}
//		if err != test.want {
//			t.Errorf("#%d: got: %s want: %s", i, err, test.want)
//			continue
//		}
//
//		m, p, err := DecomposeRM(id.Public, msg)
//		if err != nil {
//			t.Fatalf("test %v: %v", i, err)
//		}
//		if m.Version != RMHeaderVersion {
//			t.Fatalf("version got %v want %v", m.Version, RMHeaderVersion)
//		}
//		if m.Command != test.cmd {
//			t.Fatalf("command got %v want %v", m.Command, test.cmd)
//		}
//		if !reflect.DeepEqual(test.i, p) {
//			t.Fatalf("corrupt %v", i)
//		}
//	}
//}

//func TestRequestInvite(t *testing.T) {
//	alice, err := zkidentity.New("Alice McMalice", "alice")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	bob, err := zkidentity.New("Bob Bobberino", "bob")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	charlie, err := zkidentity.New("Chalie Manson", "charles")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Alice and Bob are already talking to one another but Bob now decided
//	// to ask Alice if would like to invite Charlie to a key exchange.
//
//	// 1. Bob asks Alice to talk to Chalie
//	invite := RMInvite{
//		Invitee: charlie.Public,
//	}
//
//	// 2. Alice receives request, verifies Charlie's identity etc. and
//	// decides to go ahead and talk to Charlie.
//	cipherText, sharedKey, err := sntrup4591761.Encapsulate(rand.Reader,
//		&invite.Invitee.Key)
//	if err != nil {
//		t.Fatal(err)
//	}
//	// encrypt blob
//	blob, err := CreateInviteBlob(alice.Public, sharedKey)
//	if err != nil {
//		t.Fatal(err)
//	}
//	inviteReply := RMInviteReply{
//		For:        charlie.Public.Identity,
//		CipherText: cipherText[:],
//		InviteBlob: blob,
//	}
//
//	// 3. Bob receives reply and attempts to decapsulate the shared key for
//	// the lolz.
//	var ct sntrup4591761.Ciphertext
//	copy(ct[:], inviteReply.CipherText)
//	skFail, n := sntrup4591761.Decapsulate(&ct, &bob.PrivateKey)
//	if n == 1 || bytes.Equal(sharedKey[:], skFail[:]) {
//		t.Fatal("bob could decapsulate shared key")
//	}
//	rif := RMInviteForward{
//		CipherText: inviteReply.CipherText,
//		InviteBlob: inviteReply.InviteBlob,
//	}
//
//	// 4. Charlie receives forwarded invite from bob
//	var rifct sntrup4591761.Ciphertext
//	copy(rifct[:], rif.CipherText)
//	skAliceCharlie, n := sntrup4591761.Decapsulate(&rifct, &charlie.PrivateKey)
//	if n != 1 {
//		t.Fatal("charlie could not decapsulate")
//	}
//	pii, err := InviteFromBlob(rif.InviteBlob, skAliceCharlie)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if !reflect.DeepEqual(pii.Public, alice.Public) {
//		t.Fatal("not same identity")
//	}
//}

// TestDecomposeLimitsZlibDeflate tests that the decompose function limits the
// max amount that can be decompressed from an encoded message.
func TestDecomposeLimitsZlibDeflate(t *testing.T) {
	// Figure out the max valid size when we prepend the blob with a valid
	// RM message.
	validRM := `{"command":"pm"}` + "\n{}" // Valid header and message.
	maxSize := testMaxDecompressSize - len(validRM) - 1

	tests := []struct {
		name    string
		size    int
		wantErr error
	}{{
		name:    "max size decompression",
		size:    maxSize,
		wantErr: nil,
	}, {
		name:    "one past max size decompression",
		size:    maxSize + 1,
		wantErr: errLimitedReaderExhausted,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Generate a small compressed stream that decompresses to a large
			// message.
			mb := &bytes.Buffer{}
			w, err := zlib.NewWriterLevel(mb, zlib.BestCompression)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := w.Write([]byte(validRM)); err != nil {
				t.Fatal(err)
			}
			padding := make([]byte, tc.size)
			if _, err := w.Write(padding); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			bts := mb.Bytes()

			// Sanity check the generated message is small.
			if len(bts) > 10*1024 {
				t.Fatalf("Sanity check failed: compressed message is too large: %d",
					len(bts))
			}

			// Attempt to decompress it.
			_, _, err = DecomposeRM(nil, bts, testMaxDecompressSize)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("unexpected error: got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func decodeHex32(s string) [32]byte {
	var res [32]byte
	n, err := hex.Decode(res[:], []byte(s))
	if n != 32 {
		panic("short string")
	}
	if err != nil {
		panic(err)
	}
	return res
}

func TestMetadataStatusHash(t *testing.T) {
	tests := []struct {
		name     string
		pms      PostMetadataStatus
		wantHash string
	}{{
		name:     "empty pms",
		pms:      PostMetadataStatus{},
		wantHash: "05f6ac47accd338d329cc16f6d59f3409cc8bfe76a272e1eec612e49c115145d",
	}, {
		name:     "empty v1 pms",
		pms:      PostMetadataStatus{Version: 1},
		wantHash: "8ea40918f0472ddddd8ee06fabfebcdeca0cad2fe4a069c7e4172b819c2ee507",
	}, {
		name:     "big version",
		pms:      PostMetadataStatus{Version: 0xac5be174813d6559},
		wantHash: "77c4e7a2c54f60f73b742f2cc654dc3e1770a863b76cfd9b10dfbb06179dd449",
	}, {
		name:     "v1 with from",
		pms:      PostMetadataStatus{Version: 1, From: "0001020304"},
		wantHash: "b03bf75296affbbaaeeba07bacf239060737406373d237707e0ea8f415d3d077",
	}, {
		name: "v1 with identifier",
		pms: PostMetadataStatus{
			Version:    1,
			Attributes: map[string]string{RMPIdentifier: "000102030405"},
		},
		wantHash: "1449ad9e411f8bb5c7680bd7c3721d04aff728cc48c7261b6211cb20d237500f",
	}, {
		name: "v1 with comment",
		pms: PostMetadataStatus{
			Version:    1,
			Attributes: map[string]string{RMPSComment: "comment ウェブの国際化"},
		},
		wantHash: "b6e4860f17c2be85c95a0d2fbd1175febbb32dbbd298ec6df25ead012d82b61e",
	}, {
		name: "v1 with comment and identifier",
		pms: PostMetadataStatus{
			Version: 1,
			Attributes: map[string]string{
				RMPIdentifier: "000102030405",
				RMPSComment:   "comment ウェブの国際化",
			},
		},
		wantHash: "e280e0cd9347f3ec8a29e1b9e80c634d92ca9eb6557eac88651edcf183e7a45d",
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotHash := tc.pms.Hash()
			wantHash := decodeHex32(tc.wantHash)
			if gotHash != wantHash {
				t.Fatalf("Unexpected hash: got %x, want %x",
					gotHash, wantHash)
			}
		})
	}
}
