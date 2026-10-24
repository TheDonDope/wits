package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"gopkg.in/yaml.v3"
)

// Dir is the name of the repository directory, in the spirit of .git.
const Dir = ".wits"

const (
	configFile   = "config.yml"
	productsFile = "products.yml"
	devicesFile  = "devices.yml"
	journalFile  = "journal.ndjson"
	effectsFile  = "effects.ndjson"
	pricesFile   = "prices.ndjson"
	// remoteKeyFile is the age identity push and pull encrypt to.
	remoteKeyFile = "remote.key"
	// sealKeyFile is the Ed25519 key seals are signed with, and sealsDir
	// where they are kept.
	sealKeyFile = "seal.key"
	sealsDir    = "seals"
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
//
// A log_level once sat beside log_file and was read by nothing — the same
// dead setting the old .env had, moved to a new home. log_file went the same
// way: it named wits.log, which every launch opened and nothing ever wrote to.
// A yaml field nothing uses is a promise the program does not keep; one an
// older build wrote is ignored on reading.
//
// Theme names the interface's colour theme; empty means follow the terminal.
// Remote is the folder wits push and wits pull share with other devices.
// LeadDays is how long the pharmacy takes to fill a prescription; unset, it
// is DefaultLead.
type Config struct {
	Version  int    `yaml:"version"`
	Theme    string `yaml:"theme,omitempty"`
	Remote   string `yaml:"remote,omitempty"`
	LeadDays int    `yaml:"lead_days,omitempty"`
}

// DefaultLead is the pharmacy's lead time, in days, until one is given.
const DefaultLead = 3

// Lead is the pharmacy's lead time in days.
func (c Config) Lead() int {
	if c.LeadDays > 0 {
		return c.LeadDays
	}
	return DefaultLead
}

// DefaultConfig returns the configuration a freshly initialised repository gets.
func DefaultConfig() Config {
	return Config{Version: Version}
}

// Repo is an initialised .wits repository.
type Repo struct {
	root    string
	Config  Config
	journal *journal.Journal
	effects *effects.Log
	prices  *prices.Log
}

// Init creates a repository under dir and returns it. It refuses to touch an
// existing repository rather than overwriting one.
func Init(dir string) (*Repo, error) {
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

	if err := os.MkdirAll(root, dirPerm); err != nil {
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

// ConfigPath returns the path of the configuration.
func (r *Repo) ConfigPath() string { return filepath.Join(r.root, configFile) }

// ProductsPath returns the path of the product catalog.
func (r *Repo) ProductsPath() string { return filepath.Join(r.root, productsFile) }

// DevicesPath returns the path of the device catalog.
func (r *Repo) DevicesPath() string { return filepath.Join(r.root, devicesFile) }

// JournalPath returns the path of the event journal.
func (r *Repo) JournalPath() string { return filepath.Join(r.root, journalFile) }

// Journal returns the repository's event journal. The same instance is
// returned every time: the journal's mutex and its cached tip only mean
// something if every caller in the process appends through one value.
func (r *Repo) Journal() *journal.Journal {
	if r.journal == nil {
		r.journal = journal.Open(r.JournalPath())
	}
	return r.journal
}

// EffectsPath returns the path of the effect log.
func (r *Repo) EffectsPath() string { return filepath.Join(r.root, effectsFile) }

// Effects returns the repository's effect log — the same instance every time,
// as the journal is, so its mutex means something.
func (r *Repo) Effects() *effects.Log {
	if r.effects == nil {
		r.effects = effects.Open(r.EffectsPath())
	}
	return r.effects
}

// PricesPath returns the path of the price log.
func (r *Repo) PricesPath() string { return filepath.Join(r.root, pricesFile) }

// Prices returns the repository's price log, the same instance every time,
// as Journal does.
func (r *Repo) Prices() *prices.Log {
	if r.prices == nil {
		r.prices = prices.Open(r.PricesPath())
	}
	return r.prices
}

// SetTheme records the interface theme and persists it, under the config's
// lock and onto the config as it stands on disk, so a setting changed by
// another process in the meantime is kept rather than written over.
func (r *Repo) SetTheme(name string) error {
	return r.setConfig(func(c *Config) { c.Theme = name })
}

// SetRemote records the folder push and pull go to, the way SetTheme records
// the theme.
func (r *Repo) SetRemote(dir string) error {
	return r.setConfig(func(c *Config) { c.Remote = dir })
}

// SealKeyPath is the key seals are signed with: one's own, kept here and
// never in a bundle or a remote.
func (r *Repo) SealKeyPath() string { return filepath.Join(r.root, sealKeyFile) }

// SealsDir is where the seals of the journal's head are kept.
func (r *Repo) SealsDir() string { return filepath.Join(r.root, sealsDir) }

// SetLead records the pharmacy's lead time, in days.
func (r *Repo) SetLead(days int) error {
	return r.setConfig(func(c *Config) { c.LeadDays = days })
}

// RemoteKeyPath is the age identity the remote is encrypted to. It lives in
// the repository and never in the remote: whoever holds the remote's folder
// holds nothing without it.
func (r *Repo) RemoteKeyPath() string { return filepath.Join(r.root, remoteKeyFile) }

// setConfig changes the config under its lock, onto the config as it stands.
func (r *Repo) setConfig(change func(*Config)) error {
	path := filepath.Join(r.root, configFile)
	release, err := journal.Lock(path)
	if err != nil {
		return err
	}
	defer release()
	fresh, err := open(r.root)
	if err != nil {
		return err
	}
	r.Config = fresh.Config
	change(&r.Config)
	return r.writeConfig()
}

// writeConfig persists the configuration.
func (r *Repo) writeConfig() error {
	data, err := yaml.Marshal(r.Config)
	if err != nil {
		return err
	}
	return journal.WriteFile(filepath.Join(r.root, configFile), data)
}

// UpdateProducts changes the product catalog: under its lock, it reads the
// catalog as it stands on disk, applies change, and writes it back whole.
// The catalog it wrote is returned, for the caller to adopt.
//
// The catalogs are files read, changed and written back. Done from a copy
// read earlier — the TUI's, say, read when it opened — the write puts back
// everything else as it was then, and a product added by wits buy in another
// terminal meanwhile is gone, its entries left naming a slug nothing defines.
// A change that returns an error writes nothing.
func (r *Repo) UpdateProducts(change func(*catalog.Catalog) error) (*catalog.Catalog, error) {
	return update(r.ProductsPath(), catalog.Load, change)
}

// UpdateDevices is UpdateProducts for the device catalog.
func (r *Repo) UpdateDevices(change func(*catalog.Devices) error) (*catalog.Devices, error) {
	return update(r.DevicesPath(), catalog.LoadDevices, change)
}

// update is the read-change-write of a catalog file, under its lock.
func update[T interface{ Save(string) error }](path string, load func(string) (T, error), change func(T) error) (T, error) {
	var zero T
	release, err := journal.Lock(path)
	if err != nil {
		return zero, err
	}
	defer release()
	fresh, err := load(path)
	if err != nil {
		return zero, err
	}
	if err := change(fresh); err != nil {
		return zero, err
	}
	if err := fresh.Save(path); err != nil {
		return zero, err
	}
	return fresh, nil
}
