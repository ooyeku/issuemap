package services

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ooyeku/issuemap/internal/domain/entities"
	"github.com/ooyeku/issuemap/internal/domain/errors"
	"github.com/ooyeku/issuemap/internal/domain/repositories"
)

// DeduplicationService handles file deduplication and reference management
type DeduplicationService struct {
	basePath    string
	configRepo  repositories.ConfigRepository
	config      *entities.DeduplicationConfig
	mu          sync.RWMutex
	hashIndex   map[string]*entities.FileHash
	references  map[string][]*entities.FileReference
	indexLoaded bool
}

// NewDeduplicationService creates a new deduplication service
func NewDeduplicationService(basePath string, configRepo repositories.ConfigRepository) *DeduplicationService {
	config := entities.DefaultDeduplicationConfig()

	// Try to load config from repository
	if configRepo != nil {
		if cfg, err := configRepo.Load(context.Background()); err == nil && cfg != nil {
			if cfg.StorageConfig != nil && cfg.StorageConfig.CleanupConfig != nil {
				// For now, we'll store dedup config in storage config
				// TODO: Add dedup config to main config
			}
		}
	}

	service := &DeduplicationService{
		basePath:   basePath,
		configRepo: configRepo,
		config:     config,
		hashIndex:  make(map[string]*entities.FileHash),
		references: make(map[string][]*entities.FileReference),
	}

	return service
}

// GetConfig returns current deduplication configuration
func (d *DeduplicationService) GetConfig() *entities.DeduplicationConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// UpdateConfig updates deduplication configuration
func (d *DeduplicationService) UpdateConfig(config *entities.DeduplicationConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.config = config

	// TODO: Save to config repository
	return nil
}

// CalculateFileHash calculates the hash of a file
func (d *DeduplicationService) CalculateFileHash(reader io.Reader) (string, int64, error) {
	var hasher hash.Hash

	switch d.config.HashAlgorithm {
	case "sha256":
		hasher = sha256.New()
	case "sha1":
		hasher = sha1.New()
	case "md5":
		hasher = md5.New()
	default:
		hasher = sha256.New() // Default to SHA-256
	}

	size, err := io.Copy(hasher, reader)
	if err != nil {
		return "", 0, errors.Wrap(err, "DeduplicationService.CalculateFileHash", "failed to hash file")
	}

	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return hashString, size, nil
}

// CalculateFileHashFromPath calculates the hash of a file by path
func (d *DeduplicationService) CalculateFileHashFromPath(filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, errors.Wrap(err, "DeduplicationService.CalculateFileHashFromPath", "failed to open file")
	}
	defer file.Close()

	return d.CalculateFileHash(file)
}

// ShouldDeduplicate checks if a file should be deduplicated based on config
func (d *DeduplicationService) ShouldDeduplicate(size int64, contentType string) bool {
	if !d.config.Enabled {
		return false
	}

	// Check size limits
	if d.config.MinFileSize > 0 && size < d.config.MinFileSize {
		return false
	}

	if d.config.MaxFileSize > 0 && size > d.config.MaxFileSize {
		return false
	}

	// Check excluded types
	for _, excludedType := range d.config.ExcludedTypes {
		if strings.Contains(contentType, excludedType) {
			return false
		}
	}

	return true
}

// GetOrCreateFileHash gets existing file hash or creates new one
func (d *DeduplicationService) GetOrCreateFileHash(hash string, size int64, filename, contentType string) (*entities.FileHash, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureIndexLoaded(); err != nil {
		return nil, false, err
	}

	// Check if hash already exists
	if existing, exists := d.hashIndex[hash]; exists {
		// Update access time
		existing.LastAccessed = time.Now()
		d.saveFileHash(existing)
		return existing, false, nil
	}

	// Create new file hash entry
	fileHash := &entities.FileHash{
		Hash:             hash,
		Algorithm:        d.config.HashAlgorithm,
		Size:             size,
		OriginalFilename: filename,
		ContentType:      contentType,
		StoragePath:      d.getHashStoragePath(hash),
		RefCount:         0,
		CreatedAt:        time.Now(),
		LastAccessed:     time.Now(),
	}

	d.hashIndex[hash] = fileHash
	if err := d.saveFileHash(fileHash); err != nil {
		return nil, false, err
	}

	return fileHash, true, nil
}

// AddReference adds a reference to a deduplicated file
func (d *DeduplicationService) AddReference(attachmentID string, issueID entities.IssueID, fileHash, filename string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureIndexLoaded(); err != nil {
		return err
	}

	// Update reference count
	if hashEntry, exists := d.hashIndex[fileHash]; exists {
		hashEntry.RefCount++
		hashEntry.LastAccessed = time.Now()
		if err := d.saveFileHash(hashEntry); err != nil {
			return err
		}
	}

	// Create reference
	ref := &entities.FileReference{
		AttachmentID: attachmentID,
		IssueID:      issueID,
		FileHash:     fileHash,
		Filename:     filename,
		CreatedAt:    time.Now(),
	}

	// Add to references map
	d.references[fileHash] = append(d.references[fileHash], ref)

	// Save reference
	return d.saveFileReference(ref)
}

// RemoveReference removes a reference to a deduplicated file
func (d *DeduplicationService) RemoveReference(attachmentID string, fileHash string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureIndexLoaded(); err != nil {
		return err
	}

	// Remove from references map
	if refs, exists := d.references[fileHash]; exists {
		newRefs := make([]*entities.FileReference, 0, len(refs))
		for _, ref := range refs {
			if ref.AttachmentID != attachmentID {
				newRefs = append(newRefs, ref)
			} else {
				// Delete reference file
				d.deleteFileReference(ref)
			}
		}
		d.references[fileHash] = newRefs
	}

	// Update reference count
	if hashEntry, exists := d.hashIndex[fileHash]; exists {
		hashEntry.RefCount--
		if hashEntry.RefCount <= 0 {
			// No more references, can delete the file
			if err := d.deleteHashedFile(hashEntry); err != nil {
				return err
			}
			delete(d.hashIndex, fileHash)
			delete(d.references, fileHash)
		} else {
			if err := d.saveFileHash(hashEntry); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetDeduplicationStats calculates deduplication statistics
func (d *DeduplicationService) GetDeduplicationStats() (*entities.DeduplicationStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if err := d.ensureIndexLoaded(); err != nil {
		return nil, err
	}

	stats := &entities.DeduplicationStats{
		UniqueFiles:     len(d.hashIndex),
		TotalReferences: 0,
		UniqueSize:      0,
	}

	var topDuplicates []entities.FileHashInfo

	for hash, fileHash := range d.hashIndex {
		refCount := len(d.references[hash])
		stats.TotalReferences += refCount
		stats.UniqueSize += fileHash.Size

		// Calculate total size without deduplication
		stats.TotalSizeWithoutDedup += fileHash.Size * int64(refCount)

		// Track deduplicated files
		if refCount > 1 {
			stats.DeduplicatedFiles++

			spaceSaved := fileHash.Size * int64(refCount-1)
			topDuplicates = append(topDuplicates, entities.FileHashInfo{
				Hash:             hash,
				OriginalFilename: fileHash.OriginalFilename,
				Size:             fileHash.Size,
				RefCount:         refCount,
				SpaceSaved:       spaceSaved,
				CreatedAt:        fileHash.CreatedAt,
			})
		}
	}

	// Sort top duplicates by space saved
	sort.Slice(topDuplicates, func(i, j int) bool {
		return topDuplicates[i].SpaceSaved > topDuplicates[j].SpaceSaved
	})

	// Keep top 10
	if len(topDuplicates) > 10 {
		topDuplicates = topDuplicates[:10]
	}

	stats.TopDuplicates = topDuplicates
	stats.SpaceSaved = stats.TotalSizeWithoutDedup - stats.UniqueSize

	if stats.TotalSizeWithoutDedup > 0 {
		stats.DeduplicationRatio = float64(stats.SpaceSaved) / float64(stats.TotalSizeWithoutDedup)
	}

	return stats, nil
}

// FindPotentialDuplicates analyzes existing files for deduplication opportunities
func (d *DeduplicationService) FindPotentialDuplicates(attachmentsPath string) ([]entities.DuplicateGroup, error) {
	hashGroups := make(map[string][]entities.DuplicateFile)

	err := filepath.Walk(attachmentsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Calculate hash
		hash, size, err := d.CalculateFileHashFromPath(path)
		if err != nil {
			return nil // Skip files we can't hash
		}

		// Check if we should deduplicate this file
		if !d.ShouldDeduplicate(size, "") {
			return nil
		}

		// Extract issue ID and attachment info from path
		relPath, _ := filepath.Rel(attachmentsPath, path)
		pathParts := strings.Split(relPath, string(filepath.Separator))
		if len(pathParts) < 2 {
			return nil
		}

		issueID := pathParts[0]
		filename := pathParts[len(pathParts)-1]

		duplicateFile := entities.DuplicateFile{
			AttachmentID: filename, // Simplified for now
			IssueID:      entities.IssueID(issueID),
			StoragePath:  path,
			Filename:     filename,
			UploadedAt:   info.ModTime(),
		}

		hashGroups[hash] = append(hashGroups[hash], duplicateFile)
		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "DeduplicationService.FindPotentialDuplicates", "failed to walk attachments")
	}

	// Convert to duplicate groups
	var groups []entities.DuplicateGroup
	for hash, files := range hashGroups {
		if len(files) > 1 {
			// Calculate space savings
			fileSize := int64(0)
			if len(files) > 0 {
				if info, err := os.Stat(files[0].StoragePath); err == nil {
					fileSize = info.Size()
				}
			}

			group := entities.DuplicateGroup{
				Hash:         hash,
				Size:         fileSize,
				Count:        len(files),
				SpaceSavings: fileSize * int64(len(files)-1),
				Files:        files,
			}
			groups = append(groups, group)
		}
	}

	// Sort by space savings
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].SpaceSavings > groups[j].SpaceSavings
	})

	return groups, nil
}

// getHashStoragePath returns the storage path for a given hash
func (d *DeduplicationService) getHashStoragePath(hash string) string {
	// Use first 2 characters for directory structure to avoid too many files in one directory
	dir := hash[:2]
	filename := hash[2:]
	return filepath.Join(d.basePath, "dedup", dir, filename)
}

// ensureIndexLoaded ensures the hash index is loaded
func (d *DeduplicationService) ensureIndexLoaded() error {
	if d.indexLoaded {
		return nil
	}

	if err := d.loadIndex(); err != nil {
		return err
	}

	d.indexLoaded = true
	return nil
}

// loadIndex loads the hash index from disk
func (d *DeduplicationService) loadIndex() error {
	indexPath := filepath.Join(d.basePath, "dedup", "index")

	// Load file hashes
	hashesPath := filepath.Join(indexPath, "hashes")
	if err := filepath.Walk(hashesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		var fileHash entities.FileHash
		if err := json.Unmarshal(data, &fileHash); err != nil {
			return nil // Skip invalid files
		}

		d.hashIndex[fileHash.Hash] = &fileHash
		return nil
	}); err != nil {
		// Index directory might not exist yet
		os.MkdirAll(hashesPath, 0755)
	}

	// Load references
	refsPath := filepath.Join(indexPath, "references")
	if err := filepath.Walk(refsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		var ref entities.FileReference
		if err := json.Unmarshal(data, &ref); err != nil {
			return nil // Skip invalid files
		}

		d.references[ref.FileHash] = append(d.references[ref.FileHash], &ref)
		return nil
	}); err != nil {
		// References directory might not exist yet
		os.MkdirAll(refsPath, 0755)
	}

	return nil
}

// saveFileHash saves a file hash to disk
func (d *DeduplicationService) saveFileHash(fileHash *entities.FileHash) error {
	indexPath := filepath.Join(d.basePath, "dedup", "index", "hashes")
	os.MkdirAll(indexPath, 0755)

	filePath := filepath.Join(indexPath, fileHash.Hash+".json")
	data, err := json.MarshalIndent(fileHash, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// saveFileReference saves a file reference to disk
func (d *DeduplicationService) saveFileReference(ref *entities.FileReference) error {
	indexPath := filepath.Join(d.basePath, "dedup", "index", "references")
	os.MkdirAll(indexPath, 0755)

	filePath := filepath.Join(indexPath, ref.AttachmentID+".json")
	data, err := json.MarshalIndent(ref, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// deleteFileReference deletes a file reference from disk
func (d *DeduplicationService) deleteFileReference(ref *entities.FileReference) error {
	indexPath := filepath.Join(d.basePath, "dedup", "index", "references")
	filePath := filepath.Join(indexPath, ref.AttachmentID+".json")
	return os.Remove(filePath)
}

// deleteHashedFile deletes a hashed file and its metadata
func (d *DeduplicationService) deleteHashedFile(fileHash *entities.FileHash) error {
	// Delete the actual file
	if err := os.Remove(fileHash.StoragePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Delete the hash metadata
	indexPath := filepath.Join(d.basePath, "dedup", "index", "hashes")
	metaPath := filepath.Join(indexPath, fileHash.Hash+".json")
	return os.Remove(metaPath)
}
