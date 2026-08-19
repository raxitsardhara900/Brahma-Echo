package profiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pinchtab/pinchtab/internal/activity"
	"github.com/pinchtab/pinchtab/internal/bridge"
)

type ProfileManager struct {
	baseDir        string
	activity       activity.Recorder
	instanceLookup func(profileID string) (instanceID string, running bool)
	lockOwner      func(dir string) (owned bool, pid int)
	mu             sync.RWMutex
}

type ProfileMeta struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	UseWhen     string `json:"useWhen,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProfileDetailedInfo struct {
	ID                string    `json:"id,omitempty"`
	Name              string    `json:"name"`
	Path              string    `json:"path"`
	CreatedAt         time.Time `json:"createdAt"`
	SizeMB            float64   `json:"sizeMB"`
	Source            string    `json:"source,omitempty"`
	ChromeProfileName string    `json:"chromeProfileName,omitempty"`
	AccountEmail      string    `json:"accountEmail,omitempty"`
	AccountName       string    `json:"accountName,omitempty"`
	HasAccount        bool      `json:"hasAccount,omitempty"`
	UseWhen           string    `json:"useWhen,omitempty"`
	Description       string    `json:"description,omitempty"`
}

func NewProfileManager(baseDir string) *ProfileManager {
	_ = os.MkdirAll(baseDir, 0755)
	return &ProfileManager{
		baseDir:   baseDir,
		lockOwner: bridge.ProfileOwnedByRunningPinchtab,
	}
}

func (pm *ProfileManager) SetActivityRecorder(rec activity.Recorder) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.activity = rec
}

// ErrProfileInUse indicates a destructive operation was refused because a
// running instance holds the profile.
var ErrProfileInUse = errors.New("profile in use")

// SetInstanceLookup installs the orchestrator's profile→instance mapping at
// composition time. Without it (a mux serving no orchestrator), profileHolder
// falls back to the pinchtab.pid lock so the guard refuses on both surfaces.
func (pm *ProfileManager) SetInstanceLookup(lookup func(profileID string) (instanceID string, running bool)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.instanceLookup = lookup
}

// profileHolder is the single owner of the in-use rule: the Delete and Reset
// guards and the published running flag must agree, so all of them consult
// this and nothing else. Callers hold pm.mu.
//
// The two sources are ORed, not tried in order. An orchestrator only knows the
// instances IT started, so a profile held by a second pinchtab — another server,
// a `pinchtab bridge`, an always-on instance outside this map — makes the lookup
// answer not-running with full confidence. Returning there would delete a live
// profile on the very surface this guard was written for. The pid lock is
// per-directory truth and sees that holder; it reads not-owned for our own pid,
// which is what keeps the orchestrator's own temp-profile cleanup working.
func (pm *ProfileManager) profileHolder(id, dir string) (holder string, held bool) {
	if pm.instanceLookup != nil {
		if instanceID, running := pm.instanceLookup(id); running {
			return instanceID, true
		}
	}
	if pm.lockOwner != nil {
		if owned, pid := pm.lockOwner(dir); owned {
			return fmt.Sprintf("pinchtab process %d", pid), true
		}
	}
	return "", false
}

func (pm *ProfileManager) findProfileDirByName(name string) (string, error) {
	direct := filepath.Join(pm.baseDir, name)
	if info, err := os.Stat(direct); err == nil && info.IsDir() {
		return direct, nil
	}

	entries, err := os.ReadDir(pm.baseDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(pm.baseDir, entry.Name())
		if entry.Name() == profileID(name) {
			return dir, nil
		}
		if trustedProfileMeta(pm.baseDir, entry.Name()).Name == name {
			return dir, nil
		}
	}
	return "", fmt.Errorf("profile %q not found", name)
}

func (pm *ProfileManager) profileDir(name string) (string, error) {
	if err := ValidateProfileName(name); err != nil {
		return "", err
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.findProfileDirByName(name)
}

func (pm *ProfileManager) Exists(name string) bool {
	_, err := pm.profileDir(name)
	return err == nil
}

func (pm *ProfileManager) ProfilePath(name string) (string, error) {
	return pm.profileDir(name)
}

func (pm *ProfileManager) List() ([]bridge.ProfileInfo, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	entries, err := os.ReadDir(pm.baseDir)
	if err != nil {
		return nil, err
	}

	profiles := []bridge.ProfileInfo{}
	skip := map[string]bool{"bin": true, "profiles": true}
	for _, entry := range entries {
		if !entry.IsDir() || skip[entry.Name()] {
			continue
		}
		info, err := pm.profileInfo(entry.Name())
		if err != nil {
			continue
		}

		if _, err := os.Stat(filepath.Join(pm.baseDir, entry.Name(), "Default")); err != nil {
			continue
		}

		pathExists := true
		if _, err := os.Stat(info.Path); err != nil {
			pathExists = false
		}

		_, held := pm.profileHolder(info.ID, filepath.Join(pm.baseDir, entry.Name()))

		profiles = append(profiles, bridge.ProfileInfo{
			ID:                info.ID,
			Name:              info.Name,
			Path:              info.Path,
			PathExists:        pathExists,
			Running:           held,
			Created:           info.CreatedAt,
			Temporary:         strings.HasPrefix(info.Name, "instance-"),
			Quarantined:       bridge.IsQuarantinedProfileDir(entry.Name()),
			DiskUsage:         int64(info.SizeMB * 1024 * 1024),
			Source:            info.Source,
			ChromeProfileName: info.ChromeProfileName,
			AccountEmail:      info.AccountEmail,
			AccountName:       info.AccountName,
			HasAccount:        info.HasAccount,
			UseWhen:           info.UseWhen,
			Description:       info.Description,
		})
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

// trustedProfileMeta reads dirName's profile.json and keeps it only for a
// directory the metadata actually owns: the profile's own name, or the ID
// derived from it, which is what create and import use. Quarantine renames a
// directory and carries profile.json along, so the file then names the profile
// the directory used to be; trusting it would give both directories one name,
// and ProfileID hashes the name, so one ID. Falling back to the directory name
// keeps them distinct, since directory names are unique within baseDir.
func trustedProfileMeta(baseDir, dirName string) ProfileMeta {
	meta := readProfileMeta(filepath.Join(baseDir, dirName))
	if meta.Name == "" || dirName == meta.Name || dirName == profileID(meta.Name) {
		return meta
	}
	return ProfileMeta{}
}

func (pm *ProfileManager) profileInfo(dirName string) (ProfileDetailedInfo, error) {
	if err := ValidateProfileName(dirName); err != nil {
		return ProfileDetailedInfo{}, err
	}
	dir := filepath.Join(pm.baseDir, dirName)
	fi, err := os.Stat(dir)
	if err != nil {
		return ProfileDetailedInfo{}, err
	}

	size := dirSizeMB(dir)
	source := "created"
	if _, err := os.Stat(filepath.Join(dir, ".pinchtab-imported")); err == nil {
		source = "imported"
	}

	chromeProfileName, accountEmail, accountName, hasAccount := readChromeProfileIdentity(dir)
	meta := trustedProfileMeta(pm.baseDir, dirName)
	profileName := meta.Name
	if profileName == "" {
		profileName = dirName
	}

	// Backfill in memory only — read paths (List/Get) must not mutate the
	// filesystem. profileID is deterministic and FindByID tolerates an empty
	// persisted ID, so persistence belongs to create/import/rename, not reads.
	if meta.ID == "" {
		meta.ID = profileID(profileName)
	}
	if meta.Name == "" {
		meta.Name = profileName
	}

	return ProfileDetailedInfo{
		ID:                meta.ID,
		Name:              profileName,
		Path:              dir,
		CreatedAt:         fi.ModTime(),
		SizeMB:            size,
		Source:            source,
		ChromeProfileName: chromeProfileName,
		AccountEmail:      accountEmail,
		AccountName:       accountName,
		HasAccount:        hasAccount,
		UseWhen:           meta.UseWhen,
		Description:       meta.Description,
	}, nil
}

func (pm *ProfileManager) FindByID(id string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	entries, err := os.ReadDir(pm.baseDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta := trustedProfileMeta(pm.baseDir, entry.Name())
		if meta.ID == id {
			if meta.Name != "" {
				return meta.Name, nil
			}
			return entry.Name(), nil
		}
		if entry.Name() == id && meta.Name != "" {
			return meta.Name, nil
		}
		if meta.ID == "" && profileID(entry.Name()) == id {
			return entry.Name(), nil
		}
	}
	return "", fmt.Errorf("profile with id %q not found", id)
}

func dirSizeMB(path string) float64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return float64(total) / (1024 * 1024)
}
