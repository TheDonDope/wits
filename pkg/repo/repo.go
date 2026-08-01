package repo

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/TheDonDope/wits/pkg/journal"
	"gopkg.in/yaml.v3"
)

// Dir is the name of the repository directory, in the spirit of .git.
const Dir = ".wits"

const (
	configFile   = "config.yml"
	productsFile = "products.yml"
	devicesFile  = "devices.yml"
	journalFile  = "journal.ndjson"
	indexDir     = "index"
)

// Permissions are deliberately tight. This is a multi-year medical record, and
// the default umask is not good enough for it.
const (
	dirPerm  os.FileMode = 0700
	filePerm os.FileMode = 0600
)

// Version is the current repository format version.
const Version = 1

var (
	// ErrNotARepo is returned when no .wits directory can be found.
	ErrNotARepo = errors.New("not a wits repository (or any parent up to the root)")
	// ErrAlreadyARepo is returned when initialising over an existing repository.
	ErrAlreadyARepo = errors.New("a wits repository already exists here")
)

// Config holds the repository settings. It replaces the environment variables
// the application used to require in a .env file.
type Config struct {
	Version  int    `yaml:"version"`
	LogLevel string `yaml:"log_level"`
	LogFile  string `yaml:"log_file"`
}

// DefaultConfig returns the configuration a freshly initialised repository gets.
func DefaultConfig() Config {
	return Config{
		Version:  Version,
		LogLevel: "INFO",
		LogFile:  "wits.log",
	}
}

// Repo is an initialised .wits repository.
type Repo struct {
	root   string
	Config Config
}

// Init creates a repository under dir and returns it. It refuses to touch an
// existing repository rather than overwriting one.
func Init(dir string) (*Repo, error) {
	log.Printf("💬 🗄️  (pkg/repo/repo.go) Init(dir string: %v) \n", dir)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(abs, Dir)
	if _, err := os.Stat(root); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyARepo, root)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Join(root, indexDir), dirPerm); err != nil {
		return nil, err
	}

	r := &Repo{root: root, Config: DefaultConfig()}
	if err := r.writeConfig(); err != nil {
		return nil, err
	}
	for _, seed := range []struct {
		path    string
		content string
	}{
		{r.ProductsPath(), "# Products dispensed to you. Managed by `wits buy`.\nproducts: []\n"},
		{r.DevicesPath(), "# Vaporizers and their temperature ranges.\ndevices: []\n"},
	} {
		if err := os.WriteFile(seed.path, []byte(seed.content), filePerm); err != nil {
			return nil, err
		}
	}

	log.Printf("✅ 🗄️  (pkg/repo/repo.go) Init() -> root: %v \n", root)
	return r, nil
}

// Discover walks up from start looking for a repository, the way git finds its
// own directory.
func Discover(start string) (*Repo, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for dir := abs; ; {
		root := filepath.Join(dir, Dir)
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			return open(root)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNotARepo
		}
		dir = parent
	}
}

// open loads the configuration of the repository rooted at root.
func open(root string) (*Repo, error) {
	r := &Repo{root: root, Config: DefaultConfig()}
	data, err := os.ReadFile(filepath.Join(root, configFile))
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, &r.Config); err != nil {
		return nil, fmt.Errorf("reading %s: %w", configFile, err)
	}
	if r.Config.Version > Version {
		return nil, fmt.Errorf("repository is format version %d, this build understands %d", r.Config.Version, Version)
	}
	return r, nil
}

// Root returns the absolute path of the .wits directory.
func (r *Repo) Root() string { return r.root }

// WorkTree returns the directory the repository lives in.
func (r *Repo) WorkTree() string { return filepath.Dir(r.root) }

// ProductsPath returns the path of the product catalog.
func (r *Repo) ProductsPath() string { return filepath.Join(r.root, productsFile) }

// DevicesPath returns the path of the device catalog.
func (r *Repo) DevicesPath() string { return filepath.Join(r.root, devicesFile) }

// JournalPath returns the path of the event journal.
func (r *Repo) JournalPath() string { return filepath.Join(r.root, journalFile) }

// Journal returns the repository's event journal.
func (r *Repo) Journal() *journal.Journal { return journal.Open(r.JournalPath()) }

// writeConfig persists the configuration.
func (r *Repo) writeConfig() error {
	data, err := yaml.Marshal(r.Config)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.root, configFile), data, filePerm)
}
