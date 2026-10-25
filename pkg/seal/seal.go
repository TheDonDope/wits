// Package seal is a chain others can check: the journal's head, signed with a
// key of one's own, and optionally timestamped with OpenTimestamps, so that a
// doctor or an authority can check a report against it without trusting the
// patient or wits.
//
// A seal is a few lines of text: the head's hash, how many entries it closes,
// when it was made, the public key, and an Ed25519 signature over the lines
// before it. The hash covers every entry before it, so a seal fixes the whole
// journal up to that point: change one gram anywhere in it and the head no
// longer matches. The signature says who sealed it; a timestamp says it
// existed by a time nobody can move — the seal's SHA-256 committed into the
// Bitcoin chain by the OpenTimestamps calendars, which receive that hash and
// nothing else. The .ots file is the standard format: `ots verify` and
// opentimestamps.org check it without wits.
package seal

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
)

// magic opens every seal; the number after it is the format.
const magic = "wits-seal 1"

var (
	// ErrForged is why a seal does not verify: its signature is not over its
	// lines, made with its key.
	ErrForged = errors.New("the signature does not match the seal: it was changed, or not made with this key")
	// ErrNoKey is why there is nothing to seal with yet.
	ErrNoKey = errors.New("no sealing key yet: `wits seal` makes one")
)

// Seal is the journal's head, signed.
type Seal struct {
	Head    string
	Entries int
	At      time.Time
	Key     ed25519.PublicKey
	Sig     []byte
}

// body is the lines the signature is over.
func (s Seal) body() string {
	return fmt.Sprintf("%s\nhead %s\nentries %d\nat %s\nkey ed25519:%s\n", magic, s.Head, s.Entries,
		s.At.UTC().Format(time.RFC3339), base64.StdEncoding.EncodeToString(s.Key))
}

// String is the seal as it is written and handed over.
func (s Seal) String() string {
	return s.body() + "sig " + base64.StdEncoding.EncodeToString(s.Sig) + "\n"
}

// Name is the seal's file name, without its extension: the day it was made,
// in UTC as the seal writes its time, and the start of the head it seals.
func (s Seal) Name() string {
	return s.At.UTC().Format(time.DateOnly) + "-" + s.Head[:min(12, len(s.Head))]
}

// Digest is the SHA-256 of the seal as written: what is timestamped.
func (s Seal) Digest() [32]byte { return sha256.Sum256([]byte(s.String())) }

// Verify reports whether the signature is over the seal's lines, by its key.
func (s Seal) Verify() error {
	if len(s.Key) != ed25519.PublicKeySize || !ed25519.Verify(s.Key, []byte(s.body()), s.Sig) {
		return ErrForged
	}
	return nil
}

// Fingerprint names a public key the way ssh does: SHA256 and its hash,
// short enough to read out over the phone or print on a report.
func Fingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// Make seals the journal's head with the key.
func Make(key ed25519.PrivateKey, head string, entries int, at time.Time) Seal {
	s := Seal{Head: head, Entries: entries, At: at.Truncate(time.Second), Key: key.Public().(ed25519.PublicKey)}
	s.Sig = ed25519.Sign(key, []byte(s.body()))
	return s
}

// Parse reads a seal as written.
func Parse(text string) (Seal, error) {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	if len(lines) != 6 || lines[0] != magic {
		return Seal{}, errors.New("not a wits seal")
	}
	field := func(i int, name string) (string, error) {
		v, ok := strings.CutPrefix(lines[i], name+" ")
		if !ok {
			return "", fmt.Errorf("line %d of the seal should be %s", i+1, name)
		}
		return v, nil
	}
	var s Seal
	var err error
	var v string
	if s.Head, err = field(1, "head"); err != nil {
		return s, err
	}
	if v, err = field(2, "entries"); err == nil {
		s.Entries, err = strconv.Atoi(v)
	}
	if err == nil {
		v, err = field(3, "at")
	}
	if err == nil {
		s.At, err = time.Parse(time.RFC3339, v)
	}
	if err == nil {
		v, err = field(4, "key")
	}
	if err == nil {
		s.Key, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(v, "ed25519:"))
	}
	if err == nil {
		v, err = field(5, "sig")
	}
	if err == nil {
		s.Sig, err = base64.StdEncoding.DecodeString(v)
	}
	if err != nil {
		return Seal{}, fmt.Errorf("unreadable seal: %w", err)
	}
	return s, nil
}

// LoadKey reads the sealing key kept at path.
func LoadKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s is not a sealing key", path)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// Key reads the sealing key at path, making one the first time.
//
// Reading and making happen under the key's lock: two first uses at once —
// the browser's carry page and wits seal — each made a key, and the one
// written second replaced the first, so whatever the first had signed could
// never be verified again.
func Key(path string) (ed25519.PrivateKey, bool, error) {
	release, err := journal.Lock(path)
	if err != nil {
		return nil, false, err
	}
	defer release()
	key, err := LoadKey(path)
	if !errors.Is(err, ErrNoKey) {
		return key, false, err
	}
	_, key, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, false, err
	}
	return key, true, journal.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key.Seed())+"\n"))
}

// File is one seal as kept: its text, and its timestamp proof when it has one.
type File struct {
	Name string
	Text string
	OTS  []byte
}

// Store is the folder seals are kept in.
type Store struct{ Dir string }

// Save writes a seal, and its proof when there is one.
func (st Store) Save(f File) error {
	if err := os.MkdirAll(st.Dir, 0700); err != nil {
		return err
	}
	if err := journal.WriteFile(filepath.Join(st.Dir, f.Name+".seal"), []byte(f.Text)); err != nil {
		return err
	}
	if len(f.OTS) == 0 {
		return nil
	}
	return journal.WriteFile(filepath.Join(st.Dir, f.Name+".ots"), f.OTS)
}

// All is every seal kept, oldest first.
func (st Store) All() ([]File, error) {
	names, err := filepath.Glob(filepath.Join(st.Dir, "*.seal"))
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	out := make([]File, 0, len(names))
	for _, n := range names {
		text, err := os.ReadFile(n)
		if err != nil {
			return nil, err
		}
		f := File{Name: strings.TrimSuffix(filepath.Base(n), ".seal"), Text: string(text)}
		if ots, err := os.ReadFile(strings.TrimSuffix(n, ".seal") + ".ots"); err == nil {
			f.OTS = ots
		}
		out = append(out, f)
	}
	return out, nil
}

// Calendars are the public OpenTimestamps calendars a seal's hash is sent to.
// Each commits it into Bitcoin on its own; one is enough.
var Calendars = []string{
	"https://a.pool.opentimestamps.org",
	"https://b.pool.opentimestamps.org",
	"https://a.pool.eternitywall.com",
	"https://ots.btc.catallaxy.com",
}

// otsMagic opens every .ots file.
var otsMagic = []byte("\x00OpenTimestamps\x00\x00Proof\x00\xbf\x89\xe2\xe8\x84\xe8\x92\x94")

// The attestation tags OpenTimestamps defines.
var (
	pendingTag = []byte{0x83, 0xdf, 0xe3, 0x0d, 0x2e, 0xf9, 0x0c, 0x8e}
	bitcoinTag = []byte{0x05, 0x88, 0x96, 0x0d, 0x73, 0xd7, 0x19, 0x01}
)

// Stamp sends the digest to the calendars and returns the .ots proof they
// make between them: pending, until a calendar's commitment is in a block.
// It succeeds when one calendar answers.
func Stamp(ctx context.Context, client *http.Client, digest [32]byte, calendars []string) ([]byte, int, error) {
	root := &stamp{}
	answered := 0
	var errs []error
	for _, c := range calendars {
		body, err := call(ctx, client, http.MethodPost, c+"/digest", digest[:])
		if err == nil {
			var t *stamp
			if t, err = readStamp(&reader{b: body}, digest[:]); err == nil {
				root.merge(t)
				answered++
				continue
			}
		}
		errs = append(errs, fmt.Errorf("%s: %w", c, err))
	}
	if answered == 0 {
		return nil, 0, fmt.Errorf("no calendar answered: %w", errors.Join(errs...))
	}
	return file(digest[:], root), answered, nil
}

// Upgrade asks each pending calendar whether its commitment has reached a
// block, and puts what it answers in place of the promise. It reports whether
// anything changed; a calendar that has nothing yet is not an error.
func Upgrade(ctx context.Context, client *http.Client, ots []byte) ([]byte, bool, error) {
	digest, root, err := readFile(ots)
	if err != nil {
		return nil, false, err
	}
	changed := false
	root.walk(digest, func(node *stamp, msg []byte) {
		for i := 0; i < len(node.attestations); i++ {
			a := node.attestations[i]
			if !bytes.Equal(a.tag, pendingTag) {
				continue
			}
			body, err := call(ctx, client, http.MethodGet, pendingURI(a.payload)+"/timestamp/"+hex.EncodeToString(msg), nil)
			if err != nil {
				continue
			}
			if t, err := readStamp(&reader{b: body}, msg); err == nil && t.attested() {
				node.attestations = slices.Delete(node.attestations, i, i+1)
				node.merge(t)
				changed = true
				i--
			}
		}
	})
	return file(digest, root), changed, nil
}

// Proof is what an .ots file attests, as far as can be told without a copy
// of the Bitcoin chain: the digest it is for, the calendars still to confirm,
// and the blocks it is in.
type Proof struct {
	Digest  []byte
	Pending []string
	Blocks  []int
}

// Describe reads what an .ots proof attests.
func Describe(ots []byte) (Proof, error) {
	digest, root, err := readFile(ots)
	if err != nil {
		return Proof{}, err
	}
	p := Proof{Digest: digest}
	root.walk(digest, func(node *stamp, _ []byte) {
		for _, a := range node.attestations {
			switch {
			case bytes.Equal(a.tag, pendingTag):
				p.Pending = append(p.Pending, pendingURI(a.payload))
			case bytes.Equal(a.tag, bitcoinTag):
				h, _ := (&reader{b: a.payload}).varuint()
				p.Blocks = append(p.Blocks, int(h))
			}
		}
	})
	return p, nil
}

// call makes one request to a calendar.
func call(ctx context.Context, client *http.Client, method, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.opentimestamps.v1")
	req.Header.Set("User-Agent", "wits")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	return data, nil
}

func pendingURI(payload []byte) string {
	uri, _ := (&reader{b: payload}).varbytes()
	return string(uri)
}

// stamp is an OpenTimestamps timestamp: what is attested about a message, and
// the operations that lead from it to further messages and what is attested
// about those.
type stamp struct {
	attestations []attestation
	ops          []step
}

type attestation struct{ tag, payload []byte }

// step is one operation and the timestamp of the message it gives.
type step struct {
	op   byte
	arg  []byte
	next *stamp
}

// The operation tags; the binary two take an argument. SHA-1 and RIPEMD-160
// are in the format too, and refused here: the calendars and Bitcoin's own
// commitments use SHA-256, and a proof that leans on a broken hash is not one
// to vouch for.
const (
	opSHA256  = 0x08
	opAppend  = 0xf0
	opPrepend = 0xf1
	opReverse = 0xf2
	opHexlify = 0xf3
)

// apply runs an operation on a message.
func apply(op byte, arg, msg []byte) ([]byte, error) {
	switch op {
	case opSHA256:
		h := sha256.Sum256(msg)
		return h[:], nil
	case opAppend:
		return append(slices.Clone(msg), arg...), nil
	case opPrepend:
		return append(slices.Clone(arg), msg...), nil
	case opReverse:
		out := slices.Clone(msg)
		slices.Reverse(out)
		return out, nil
	case opHexlify:
		return []byte(hex.EncodeToString(msg)), nil
	}
	return nil, fmt.Errorf("unsupported operation 0x%02x", op)
}

// attested reports whether a Bitcoin attestation is anywhere under the stamp.
func (t *stamp) attested() bool {
	found := false
	t.walk(nil, func(n *stamp, _ []byte) {
		for _, a := range n.attestations {
			found = found || bytes.Equal(a.tag, bitcoinTag)
		}
	})
	return found
}

// walk visits every node with the message it is a timestamp of; with a nil
// message, the messages are not worked out.
func (t *stamp) walk(msg []byte, visit func(*stamp, []byte)) {
	visit(t, msg)
	for _, s := range t.ops {
		var next []byte
		if msg != nil {
			next, _ = apply(s.op, s.arg, msg)
		}
		s.next.walk(next, visit)
	}
}

// merge adds another timestamp of the same message.
func (t *stamp) merge(o *stamp) {
	for _, a := range o.attestations {
		if !slices.ContainsFunc(t.attestations, func(b attestation) bool {
			return bytes.Equal(a.tag, b.tag) && bytes.Equal(a.payload, b.payload)
		}) {
			t.attestations = append(t.attestations, a)
		}
	}
	for _, s := range o.ops {
		i := slices.IndexFunc(t.ops, func(m step) bool { return m.op == s.op && bytes.Equal(m.arg, s.arg) })
		if i < 0 {
			t.ops = append(t.ops, s)
		} else {
			t.ops[i].next.merge(s.next)
		}
	}
}

// file is a timestamp of the digest as an .ots file.
func file(digest []byte, t *stamp) []byte {
	var b bytes.Buffer
	b.Write(otsMagic)
	b.WriteByte(0x01)
	b.WriteByte(opSHA256)
	b.Write(digest)
	t.write(&b)
	return b.Bytes()
}

// readFile reads an .ots file of a SHA-256 digest.
func readFile(ots []byte) ([]byte, *stamp, error) {
	if !bytes.HasPrefix(ots, otsMagic) || len(ots) < len(otsMagic)+2+32 {
		return nil, nil, errors.New("not an OpenTimestamps proof")
	}
	r := &reader{b: ots[len(otsMagic):]}
	if v, _ := r.varuint(); v != 1 {
		return nil, nil, fmt.Errorf("OpenTimestamps proof version %d", v)
	}
	if op, _ := r.byte(); op != opSHA256 {
		return nil, nil, errors.New("the proof is not of a SHA-256 digest")
	}
	digest, err := r.bytes(32)
	if err != nil {
		return nil, nil, err
	}
	t, err := readStamp(r, digest)
	return digest, t, err
}

// write serialises a timestamp the way OpenTimestamps does: every branch but
// the last behind a fork marker, attestations before operations.
func (t *stamp) write(b *bytes.Buffer) {
	items := len(t.attestations) + len(t.ops)
	n := 0
	fork := func() {
		n++
		if n < items {
			b.WriteByte(0xff)
		}
	}
	for _, a := range t.attestations {
		fork()
		b.WriteByte(0x00)
		b.Write(a.tag)
		writeVarbytes(b, a.payload)
	}
	for _, s := range t.ops {
		fork()
		b.WriteByte(s.op)
		if s.op == opAppend || s.op == opPrepend {
			writeVarbytes(b, s.arg)
		}
		s.next.write(b)
	}
}

// readStamp reads a timestamp of msg.
func readStamp(r *reader, msg []byte) (*stamp, error) {
	t := &stamp{}
	for {
		tag, err := r.byte()
		if err != nil {
			return nil, err
		}
		last := tag != 0xff
		if !last {
			if tag, err = r.byte(); err != nil {
				return nil, err
			}
		}
		if err := t.readItem(r, tag, msg); err != nil {
			return nil, err
		}
		if last {
			return t, nil
		}
	}
}

// readItem reads one attestation, or one operation and what follows it.
func (t *stamp) readItem(r *reader, tag byte, msg []byte) error {
	if tag == 0x00 {
		attTag, err := r.bytes(8)
		if err != nil {
			return err
		}
		payload, err := r.varbytes()
		if err != nil {
			return err
		}
		t.attestations = append(t.attestations, attestation{tag: attTag, payload: payload})
		return nil
	}
	var arg []byte
	if tag == opAppend || tag == opPrepend {
		var err error
		if arg, err = r.varbytes(); err != nil {
			return err
		}
	}
	next, err := apply(tag, arg, msg)
	if err != nil {
		return err
	}
	sub, err := readStamp(r, next)
	if err != nil {
		return err
	}
	t.ops = append(t.ops, step{op: tag, arg: arg, next: sub})
	return nil
}

// reader reads OpenTimestamps' encoding: bytes, LEB128 integers, and byte
// strings behind their length.
type reader struct {
	b []byte
	i int
}

func (r *reader) byte() (byte, error) {
	if r.i >= len(r.b) {
		return 0, io.ErrUnexpectedEOF
	}
	r.i++
	return r.b[r.i-1], nil
}

func (r *reader) bytes(n int) ([]byte, error) {
	if n < 0 || r.i+n > len(r.b) {
		return nil, io.ErrUnexpectedEOF
	}
	r.i += n
	return r.b[r.i-n : r.i], nil
}

func (r *reader) varuint() (uint64, error) {
	var v uint64
	for shift := 0; shift < 64; shift += 7 {
		c, err := r.byte()
		if err != nil {
			return 0, err
		}
		v |= uint64(c&0x7f) << shift
		if c&0x80 == 0 {
			return v, nil
		}
	}
	return 0, errors.New("varuint too long")
}

func (r *reader) varbytes() ([]byte, error) {
	n, err := r.varuint()
	if err != nil {
		return nil, err
	}
	if n > 1<<16 {
		return nil, errors.New("byte string too long")
	}
	return r.bytes(int(n))
}

func writeVarbytes(b *bytes.Buffer, p []byte) {
	n := uint64(len(p))
	for n >= 0x80 {
		b.WriteByte(byte(n) | 0x80)
		n >>= 7
	}
	b.WriteByte(byte(n))
	b.Write(p)
}

// carryMagic opens every statement of what is carried.
const carryMagic = "wits-carry 1"

// VerifyPage is where a statement's QR code points: a page that checks the
// signature in the browser, the statement after the # — which a browser
// never sends to the server it asks for the page.
const VerifyPage = "https://thedondope.github.io/wits/verify.html#"

// Carry is a signed statement of what is held, and which fills dispensed it:
// what a patient shows to prove that what they carry is what was prescribed.
type Carry struct {
	At      time.Time
	Head    string
	Entries int
	Items   []Item
	Key     ed25519.PublicKey
	Sig     []byte
}

// Item is one product held: sealed, ground and already vaped, and the fills
// it came from.
type Item struct {
	Name                string
	Sealed, Ground, AVB float64
	Fills               []Dispensed
}

// Dispensed is one fill of a product: the day the pharmacy dispensed it, and
// how much.
type Dispensed struct {
	Day   string
	Grams float64
}

func (c Carry) body() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nat %s\nhead %s %d\n", carryMagic, c.At.UTC().Format(time.RFC3339), c.Head, c.Entries)
	for _, it := range c.Items {
		fmt.Fprintf(&b, "item %.2f %.2f %.2f %s\n", it.Sealed, it.Ground, it.AVB, it.Name)
		for _, f := range it.Fills {
			fmt.Fprintf(&b, "fill %s %.2f\n", f.Day, f.Grams)
		}
	}
	fmt.Fprintf(&b, "key ed25519:%s\n", base64.StdEncoding.EncodeToString(c.Key))
	return b.String()
}

// String is the statement as it is signed and carried.
func (c Carry) String() string {
	return c.body() + "sig " + base64.StdEncoding.EncodeToString(c.Sig) + "\n"
}

// URL is the address the QR code carries: the verify page, and after the #
// the statement deflated and in base64, behind a z that says so. Deflated it
// is a third shorter, and a QR code that much smaller is that much easier to
// read off a phone's screen.
func (c Carry) URL() string {
	var b bytes.Buffer
	w, _ := flate.NewWriter(&b, flate.BestCompression) // a valid level cannot fail
	_, _ = w.Write([]byte(c.String()))
	_ = w.Close()
	return VerifyPage + "z" + base64.RawURLEncoding.EncodeToString(b.Bytes())
}

// MakeCarry signs a statement of what is held.
func MakeCarry(key ed25519.PrivateKey, head string, entries int, at time.Time, items []Item) Carry {
	c := Carry{At: at.Truncate(time.Second), Head: head, Entries: entries, Items: items, Key: key.Public().(ed25519.PublicKey)}
	c.Sig = ed25519.Sign(key, []byte(c.body()))
	return c
}

// Verify reports whether the statement's signature is over its lines, by its key.
func (c Carry) Verify() error {
	if len(c.Key) != ed25519.PublicKeySize || !ed25519.Verify(c.Key, []byte(c.body()), c.Sig) {
		return ErrForged
	}
	return nil
}

// ParseCarry reads a statement as written, or from its verify page's address.
func ParseCarry(text string) (Carry, error) {
	if _, frag, ok := strings.Cut(text, "#"); ok {
		raw, err := fragment(frag)
		if err != nil {
			return Carry{}, fmt.Errorf("unreadable statement: %w", err)
		}
		text = string(raw)
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) < 5 || lines[0] != carryMagic {
		return Carry{}, errors.New("not a wits statement of what is carried")
	}
	var c Carry
	for _, l := range lines[1:] {
		if err := c.readLine(l); err != nil {
			return Carry{}, fmt.Errorf("unreadable statement: %w", err)
		}
	}
	return c, nil
}

// fragment is the statement an address carries after its #: deflated
// behind a z, or as it is.
func fragment(frag string) ([]byte, error) {
	packed, deflated := strings.CutPrefix(frag, "z")
	raw, err := base64.RawURLEncoding.DecodeString(packed)
	if err != nil || !deflated {
		return raw, err
	}
	return io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(raw)), 1<<16))
}

// readLine reads one line of a statement into it.
func (c *Carry) readLine(l string) error {
	word, rest, _ := strings.Cut(l, " ")
	var err error
	switch word {
	case "at":
		c.At, err = time.Parse(time.RFC3339, rest)
	case "head":
		_, err = fmt.Sscanf(rest, "%s %d", &c.Head, &c.Entries)
	case "item":
		var it Item
		parts := strings.SplitN(rest, " ", 4)
		if len(parts) != 4 {
			return errors.New("an item is three amounts and a name")
		}
		_, err = fmt.Sscanf(strings.Join(parts[:3], " "), "%f %f %f", &it.Sealed, &it.Ground, &it.AVB)
		it.Name = parts[3]
		c.Items = append(c.Items, it)
	case "fill":
		if len(c.Items) == 0 {
			return errors.New("a fill before any item")
		}
		var f Dispensed
		_, err = fmt.Sscanf(rest, "%s %f", &f.Day, &f.Grams)
		last := &c.Items[len(c.Items)-1]
		last.Fills = append(last.Fills, f)
	case "key":
		c.Key, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(rest, "ed25519:"))
	case "sig":
		c.Sig, err = base64.StdEncoding.DecodeString(rest)
	default:
		err = fmt.Errorf("unknown line %q", word)
	}
	return err
}
