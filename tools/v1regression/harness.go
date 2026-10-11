package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// target: one built revision plus its scratch directory
// ---------------------------------------------------------------------------

// Target is a built revision of the project under test.
type Target struct {
	Ref        string
	Version    string
	ProjectURL string
	Bin        map[string]string
	Root       string
	Dir        string
	ExtraEnv   []string
	procs      []*Proc
}

// scoped returns a copy of the target with a fresh, scenario-specific working
// directory and an empty environment, so one scenario cannot influence another.
func (t *Target) scoped(scenario string) (*Target, error) {
	name := strings.NewReplacer("/", "_", " ", "_").Replace(scenario)
	dir := filepath.Join(t.Root, "run", t.Ref, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	clone := *t
	clone.Dir = dir
	clone.ExtraEnv = nil
	return &clone, nil
}

func (t *Target) path(name string) (string, error) {
	p, ok := t.Bin[name]
	if !ok {
		return "", fmt.Errorf("no %q binary for %s", name, t.Ref)
	}
	return p, nil
}

// env returns a deterministic child environment: nothing is inherited except PATH
// and HOME, so an ambient proxy or locale cannot make a run flaky.
func (t *Target) env() []string {
	m := map[string]string{
		"PATH": os.Getenv("PATH"),
		"HOME": os.Getenv("HOME"),
		"TZ":   "UTC",
	}
	for _, kv := range t.ExtraEnv {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(m))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

// CmdResult is the outcome of one binary invocation.
type CmdResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Run executes a binary from the target's scratch directory.
func (t *Target) Run(name string, args ...string) *CmdResult {
	return t.RunDir(t.Dir, name, args...)
}

// RunDir executes a binary with the given working directory.
func (t *Target) RunDir(dir, name string, args ...string) *CmdResult {
	bin, err := t.path(name)
	if err != nil {
		return &CmdResult{ExitCode: -1, Stderr: err.Error()}
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = t.env()

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	exit := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			exit = ee.ExitCode()
		} else {
			exit = -1
			stderr.WriteString("\n[harness] " + err.Error())
		}
	}
	return &CmdResult{ExitCode: exit, Stdout: stdout.String(), Stderr: stderr.String()}
}

// Proc is a background process (used for the server scenarios).
type Proc struct {
	cmd    *exec.Cmd
	stdout *lockedBuffer
	stderr *lockedBuffer
}

// Start launches a binary in the background.
func (t *Target) Start(name string, args ...string) (*Proc, error) {
	bin, err := t.path(name)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = t.Dir
	cmd.Env = t.env()

	p := &Proc{cmd: cmd, stdout: &lockedBuffer{}, stderr: &lockedBuffer{}}
	cmd.Stdout, cmd.Stderr = p.stdout, p.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	t.procs = append(t.procs, p)
	return p, nil
}

// stopProcs terminates any background process the scenario left behind, so a
// scenario that fails half-way cannot leak a listening server.
func (t *Target) stopProcs() {
	for _, p := range t.procs {
		p.Stop()
	}
	t.procs = nil
}

// Stop terminates the process and returns its combined output.
func (p *Proc) Stop() string {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _, _ = p.cmd.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = p.cmd.Process.Kill()
		}
	}
	return p.stdout.String() + p.stderr.String()
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// WriteFile writes a file relative to the scratch directory.
func (t *Target) WriteFile(rel, content string) {
	p := filepath.Join(t.Dir, rel)
	must(os.MkdirAll(filepath.Dir(p), 0o755))
	must(os.WriteFile(p, []byte(content), 0o600))
}

// probeSource is compiled inside each revision's own tree. It loads a config
// directory with that revision's code and prints a secret-free summary, so v1 and
// v2 acceptance of the same files can be compared field by field.
const probeSource = `package main

import (
	"fmt"
	"os"

	"ddns-watchdog/internal/client"
	"ddns-watchdog/internal/server"
)

func set(v string) string {
	if v == "" {
		return "empty"
	}
	return "set"
}

func val(v string) string {
	if v == "" {
		return "empty"
	}
	return v
}

func main() {
	client.ConfDir, server.ConfDir = os.Args[1], os.Args[2]

	err := client.Client.LoadConf()
	c := client.Client
	fmt.Printf("client       err=%v enable=ipv4:%v,ipv6:%v cycle=%d fallback=%v center=%v netcard=%v,%s,%s url=ipv4:%s,ipv6:%s,ver:%s services=dnspod:%v,alidns:%v,cf:%v,hw:%v\n",
		err, c.Enable.IPv4, c.Enable.IPv6, c.CheckCycleMinutes, c.EnableIPv6Fallback, c.Center.Enable,
		c.NetworkCard.Enable, set(c.NetworkCard.IPv4), set(c.NetworkCard.IPv6),
		set(c.APIUrl.IPv4), set(c.APIUrl.IPv6), set(c.APIUrl.Version),
		c.Services.DNSPod, c.Services.AliDNS, c.Services.Cloudflare, c.Services.HuaweiCloud)

	fmt.Printf("dnspod       err=%v id=%s token=%s domain=%s sub=%s,%s\n",
		client.DP.LoadConf(), set(client.DP.ID), set(client.DP.Token), val(client.DP.Domain),
		set(client.DP.SubDomain.A), set(client.DP.SubDomain.AAAA))
	fmt.Printf("alidns       err=%v ak=%s sk=%s domain=%s sub=%s,%s\n",
		client.AD.LoadConf(), set(client.AD.AccessKeyId), set(client.AD.AccessKeySecret), val(client.AD.Domain),
		set(client.AD.SubDomain.A), set(client.AD.SubDomain.AAAA))
	fmt.Printf("cloudflare   err=%v zone=%s token=%s domain=%s,%s proxied=%v\n",
		client.Cf.LoadConf(), set(client.Cf.ZoneID), set(client.Cf.APIToken),
		set(client.Cf.Domain.A), set(client.Cf.Domain.AAAA), client.Cf.Proxied)
	fmt.Printf("huaweicloud  err=%v ak=%s sk=%s zone=%s domain=%s,%s\n",
		client.HC.LoadConf(), set(client.HC.AccessKeyId), set(client.HC.SecretAccessKey), val(client.HC.ZoneName),
		set(client.HC.Domain.A), set(client.HC.Domain.AAAA))

	srv := &server.Srv
	fmt.Printf("server       err=%v addr=%s root=%v url=%s center=%v route=%s,%s tls=%v,%s,%s\n",
		srv.LoadConf(), val(srv.ServerAddr), srv.IsRootServer, set(srv.RootServerUrl), srv.CenterService,
		val(srv.Route.GetIP), val(srv.Route.Center), srv.TLS.Enable, set(srv.TLS.CertFile), set(srv.TLS.KeyFile))
	fmt.Printf("services     err=%v dnspod=%v,alidns=%v,cf=%v,hw=%v\n",
		server.Services.LoadConf(), server.Services.DNSPod.Enable, server.Services.AliDNS.Enable,
		server.Services.Cloudflare.Enable, server.Services.HuaweiCloud.Enable)
}
`

// writeProbe installs the probe into an extracted tree.
func writeProbe(treeDir string) error {
	dir := filepath.Join(treeDir, "tmp_probe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "main.go"), []byte(probeSource), 0o644)
}

// Probe loads clientDir/serverDir with this revision's own code and returns the
// printed summary.
func (t *Target) Probe(clientDir, serverDir string) *CmdResult {
	cmd := exec.Command("go", "run", "./tmp_probe", clientDir, serverDir)
	cmd.Dir = filepath.Join(t.Root, "src", t.Ref)
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOFLAGS=-mod=mod",
		"GOCACHE="+filepath.Join(t.Root, "gocache"),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	exit := 0
	if err := cmd.Run(); err != nil {
		exit = -1
	}
	return &CmdResult{ExitCode: exit, Stdout: stdout.String(), Stderr: stderr.String()}
}

// ---------------------------------------------------------------------------
// fake services
// ---------------------------------------------------------------------------

// FakeServer is a local HTTP service that records the requests it receives.
type FakeServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
}

// NewFakeServer starts a local HTTP server. handler may be nil.
func NewFakeServer(handler func(w http.ResponseWriter, r *http.Request, body []byte)) *FakeServer {
	fs := &FakeServer{}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fs.mu.Lock()
		fs.requests = append(fs.requests, canonicalRequest(r, body))
		fs.mu.Unlock()
		if handler != nil {
			handler(w, r, body)
		}
	}))
	return fs
}

// Requests returns the recorded requests in order.
func (fs *FakeServer) Requests() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]string(nil), fs.requests...)
}

func canonicalRequest(r *http.Request, body []byte) string {
	// the host is part of the behavior: a provider must be called at the same
	// endpoint by both revisions
	canonical := fmt.Sprintf("%s %s %s", r.Host, r.Method, r.URL.Path)
	if r.URL.RawQuery != "" {
		canonical += "?" + r.URL.RawQuery
	}
	canonical += fmt.Sprintf(" ua=%s ct=%s", r.Header.Get("User-Agent"), r.Header.Get("Content-Type"))
	if len(body) != 0 {
		canonical += " body=" + string(body)
	}
	return canonical
}

// freePort reserves a local port and returns it, so a server can be started with
// a known address.
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// waitForHTTP polls url until it answers or the deadline passes.
func waitForHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("not reachable within %s: %w", timeout, lastErr)
}

// httpDo performs a request and returns a canonical "method path -> status body"
// record, so the two revisions' replies can be compared directly.
func httpDo(method, url, body string, header map[string]string) string {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return "ERROR " + err.Error()
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "ERROR " + err.Error()
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("%s %s -> %d ct=%s body=%s",
		method, req.URL.Path, resp.StatusCode, resp.Header.Get("Content-Type"), strings.TrimSpace(string(respBody)))
}

// ---------------------------------------------------------------------------
// normalisation
// ---------------------------------------------------------------------------

var logTimestamp = regexp.MustCompile(`(?m)^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} `)

// localAddr matches the loopback addresses of the fake services, whose ports are
// random per run.
var localAddr = regexp.MustCompile(`(127\.0\.0\.1|localhost):\d+`)

// Translation maps a v2 string to the v1 wording it replaced, so that a pure
// translation does not look like a behavioral difference. Every entry here is
// taken from the v1..v2 diff and changes wording only.
type Translation struct {
	V2  string
	V1  string
	Doc string
}

// Normalize rewrites text so that only behavioral differences remain.
func (t *Target) Normalize(s string) string {
	s = logTimestamp.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, t.Dir, "<DIR>")
	if resolved, err := filepath.EvalSymlinks(t.Dir); err == nil && resolved != t.Dir {
		s = strings.ReplaceAll(s, resolved, "<DIR>")
	}
	for _, p := range t.Bin {
		s = strings.ReplaceAll(s, p, "<BIN>")
	}
	s = strings.ReplaceAll(s, t.Root, "<WORK>")
	if t.ProjectURL != "" {
		s = strings.ReplaceAll(s, t.ProjectURL, "<PROJECT_URL>")
	}
	if t.Version != "" {
		s = strings.ReplaceAll(s, t.Version, "<VER>")
	}
	s = localAddr.ReplaceAllString(s, "<ADDR>")
	for _, tr := range translations {
		s = strings.ReplaceAll(s, tr.V2, tr.V1)
	}
	// The Chinese possessive particle carries no information and v2 dropped it in a
	// few messages ("home.example.com 的 A 解析记录不存在" vs "home.example.com A
	// 解析记录不存在"). This runs after the translation table so both revisions end
	// up with the same text.
	s = strings.ReplaceAll(s, " 的 ", " ")
	return s
}

// normalizeObservation normalises every comparable field of an observation, so
// scenarios can record raw values.
func (t *Target) normalizeObservation(o *Observation) {
	o.Stdout = t.Normalize(o.Stdout)
	o.Stderr = t.Normalize(o.Stderr)
	o.ServerLog = t.Normalize(o.ServerLog)
	for i, r := range o.Requests {
		o.Requests[i] = t.Normalize(r)
	}
	for i, r := range o.Responses {
		o.Responses[i] = t.Normalize(r)
	}
	for k, v := range o.Files {
		o.Files[k] = t.Normalize(v)
	}
}

// ---------------------------------------------------------------------------
// build
// ---------------------------------------------------------------------------

var (
	versionRe    = regexp.MustCompile(`Version\s*=\s*"([^"]+)"`)
	projectURLRe = regexp.MustCompile(`projectUrl\s*=\s*"([^"]+)"`)
)

// Worktree is the ref name that builds the files currently on disk, including
// uncommitted changes, so a fix can be verified before it is committed.
const Worktree = "worktree"

// buildTarget extracts a revision with "git archive" and builds its binaries.
func buildTarget(root, ref string) (*Target, error) {
	dir := filepath.Join(root, "src", ref)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if ref == Worktree {
		if err := copyWorktree(dir); err != nil {
			return nil, err
		}
	} else if err := extractTree(ref, dir); err != nil {
		return nil, err
	}

	version, err := readVersion(dir)
	if err != nil {
		return nil, err
	}
	if err := writeProbe(dir); err != nil {
		return nil, err
	}

	binDir := filepath.Join(root, "bin", ref)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return nil, err
	}
	bin := map[string]string{}
	for _, name := range []string{"client", "server"} {
		out := filepath.Join(binDir, name)
		cmd := exec.Command("go", "build", "-o", out, "./cmd/ddns-watchdog-"+name+"/")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"GOCACHE="+filepath.Join(root, "gocache"),
		)
		if outBytes, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("building %s for %s: %v\n%s", name, ref, err, outBytes)
		}
		bin[name] = out
	}

	t := &Target{
		Ref: ref, Version: version, Bin: bin, Root: root,
		Dir:        filepath.Join(root, "run", ref),
		ProjectURL: readProjectURL(dir),
	}
	return t, os.MkdirAll(t.Dir, 0o755)
}

// extractTree pipes "git archive <ref>" into a tar reader. The working tree is
// never modified.
func extractTree(ref, dst string) error {
	cmd := exec.Command("git", "archive", ref)
	cmd.Dir = repoRoot()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { _ = cmd.Wait() }()

	tr := tar.NewReader(stdout)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", ref, err)
		}
		target := filepath.Join(dst, filepath.FromSlash(hdr.Name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyWorktree copies every tracked file of the working tree into dst, preserving
// the file modes, so uncommitted edits can be part of a comparison.
func copyWorktree(dst string) error {
	root := repoRoot()
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return err
	}
	for rel := range strings.SplitSeq(string(out), "\x00") {
		if rel == "" {
			continue
		}
		src := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(src)
		if err != nil {
			continue // deleted in the working tree
		}
		if info.IsDir() {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func readProjectURL(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "internal", "common", "common.go"))
	if err != nil {
		return ""
	}
	if m := projectURLRe.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

func readVersion(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "internal", "common", "common.go"))
	if err != nil {
		return "", err
	}
	m := versionRe.FindSubmatch(b)
	if m == nil {
		return "", fmt.Errorf("version constant not found in %s", dir)
	}
	return string(m[1]), nil
}

func repoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "."
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// TLS interception proxy (for the DNS provider APIs)
// ---------------------------------------------------------------------------

// Response is a canned reply for an intercepted request.
type Response struct {
	Status int
	Body   string
}

// Proxy terminates TLS for a fixed set of host names and answers from a script.
// Both revisions reach it through HTTPS_PROXY plus SSL_CERT_FILE, so calls to the
// hardcoded provider endpoints can be observed and compared.
type Proxy struct {
	ln       net.Listener
	caPEM    string
	caCert   *x509.Certificate
	caKey    *ecdsa.PrivateKey
	mu       sync.Mutex
	requests []string
	script   func(host, method, path string, body []byte) Response
	certs    map[string]*tls.Certificate
}

// NewProxy starts the interception proxy.
func NewProxy(script func(host, method, path string, body []byte) Response) (*Proxy, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	caTpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "v1regression-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		ln:     ln,
		caPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})),
		caCert: caCert,
		caKey:  caKey,
		script: script,
		certs:  map[string]*tls.Certificate{},
	}
	go p.serve()
	return p, nil
}

// Env returns the environment that makes a child process use the proxy.
//
// GODEBUG=x509sslcertoverrideplatform=1 is required because the default of that
// setting follows the "go" directive of the main module: a build declaring
// go < 1.27 (v1 declares 1.25) keeps the old macOS/Windows behavior of ignoring
// SSL_CERT_FILE in favor of the platform verifier. Setting it makes both
// revisions trust the harness CA, so their provider traffic can be compared.
func (p *Proxy) Env(caFile string) []string {
	return []string{
		"HTTPS_PROXY=http://" + p.ln.Addr().String(),
		"SSL_CERT_FILE=" + caFile,
		"GODEBUG=x509sslcertoverrideplatform=1",
	}
}

// WriteCA writes the CA certificate so SSL_CERT_FILE can point at it.
func (p *Proxy) WriteCA(path string) error {
	return os.WriteFile(path, []byte(p.caPEM), 0o644)
}

// Requests returns the intercepted requests in order.
func (p *Proxy) Requests() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.requests...)
}

// Close stops the proxy.
func (p *Proxy) Close() { _ = p.ln.Close() }

func (p *Proxy) serve() {
	for {
		conn, err := p.ln.Accept()
		if err != nil {
			return
		}
		go p.handle(conn)
	}
}

func (p *Proxy) handle(conn net.Conn) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	if req.Method != http.MethodConnect {
		_, _ = conn.Write([]byte("HTTP/1.1 405 Method Not Allowed\r\n\r\n"))
		return
	}
	host, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
	}

	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		return
	}

	cert, err := p.certFor(host)
	if err != nil {
		return
	}
	tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{*cert}})
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	defer tlsConn.Close()

	for {
		req, err := http.ReadRequest(bufio.NewReader(tlsConn))
		if err != nil {
			return
		}
		body, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()

		p.mu.Lock()
		p.requests = append(p.requests, canonicalRequest(req, body))
		p.mu.Unlock()

		resp := p.script(host, req.Method, req.URL.Path, body)
		if resp.Status == 0 {
			resp.Status = http.StatusOK
		}
		fmt.Fprintf(tlsConn, "HTTP/1.1 %d %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
			resp.Status, http.StatusText(resp.Status), len(resp.Body), resp.Body)
		return
	}
}

func (p *Proxy) certFor(host string) (*tls.Certificate, error) {
	p.mu.Lock()
	if c, ok := p.certs[host]; ok {
		p.mu.Unlock()
		return c, nil
	}
	p.mu.Unlock()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	if ip := net.ParseIP(host); ip != nil {
		tpl.IPAddresses = []net.IP{ip}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, p.caCert, &key.PublicKey, p.caKey)
	if err != nil {
		return nil, err
	}
	cert := &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}

	p.mu.Lock()
	p.certs[host] = cert
	p.mu.Unlock()
	return cert, nil
}
