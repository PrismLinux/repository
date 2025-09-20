package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type PackageInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	Architecture string `json:"architecture"`
	Filename     string `json:"filename"`
	Size         string `json:"size"`
	Modified     string `json:"modified"`
	Depends      string `json:"depends"`
	Groups       string `json:"groups"`
}

type RemotePackage struct {
	Filename string
	URL      string
}

type Config struct {
	RepoName      string
	RepoArchDir   string
	APIDir        string
	GitLabToken   string
	CurrentProjID string
	Commit        bool
	TestMode      bool
	CleanMode     bool
	Debug         bool
	Verbose       bool
}

// Logger provides structured logging methods
func (cfg *Config) debugLog(format string, args ...interface{}) {
	if cfg.Debug {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

func (cfg *Config) verboseLog(format string, args ...interface{}) {
	if cfg.Verbose || cfg.Debug {
		fmt.Printf("[VERBOSE] "+format+"\n", args...)
	}
}

func (cfg *Config) infoLog(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

// PackageManager handles all package management operations
type PackageManager struct {
	config       *Config
	gitlabClient *gitlab.Client
}

func NewPackageManager(cfg *Config) (*PackageManager, error) {
	pm := &PackageManager{config: cfg}

	if cfg.GitLabToken != "" && !cfg.CleanMode {
		git, err := gitlab.NewClient(cfg.GitLabToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitLab client: %w", err)
		}
		pm.gitlabClient = git
	}

	return pm, nil
}

// GitManager handles all git operations
type GitManager struct {
	config *Config
}

func NewGitManager(cfg *Config) *GitManager {
	return &GitManager{config: cfg}
}

// FileManager handles file operations
type FileManager struct {
	config *Config
}

func NewFileManager(cfg *Config) *FileManager {
	return &FileManager{config: cfg}
}

// Helper function to get string flags with defaults
func getStringFlag(cmd *cobra.Command, name, defaultValue string) string {
	if value, _ := cmd.Flags().GetString(name); value != "" {
		return value
	}
	return defaultValue
}

func NewConfig(cmd *cobra.Command) (*Config, error) {
	cfg := &Config{}

	// Set defaults and get flag values
	cfg.RepoName = getStringFlag(cmd, "repo-name", "prismlinux")
	cfg.RepoArchDir = getStringFlag(cmd, "repo-arch-dir", "public/x86_64")
	cfg.APIDir = getStringFlag(cmd, "api-dir", "public/api")

	// Handle token with fallback to environment
	cfg.GitLabToken = getStringFlag(cmd, "gitlab-token", "")
	if cfg.GitLabToken == "" {
		cfg.GitLabToken = os.Getenv("GITLAB_TOKEN")
	}

	// Handle project ID with fallback to environment
	cfg.CurrentProjID = getStringFlag(cmd, "project-id", "")
	if cfg.CurrentProjID == "" {
		cfg.CurrentProjID = os.Getenv("CI_PROJECT_ID")
	}

	cfg.Commit, _ = cmd.Flags().GetBool("commit")
	cfg.TestMode, _ = cmd.Flags().GetBool("test")
	cfg.CleanMode, _ = cmd.Flags().GetBool("clean")
	cfg.Debug, _ = cmd.Flags().GetBool("debug")
	cfg.Verbose, _ = cmd.Flags().GetBool("verbose")

	// Validate required configuration
	if cfg.GitLabToken == "" && !cfg.CleanMode {
		return nil, fmt.Errorf("GITLAB_TOKEN is required (set via flag or environment variable)")
	}

	return cfg, nil
}

var RootCmd = &cobra.Command{
	Use:   "package-manager",
	Short: "Manages the PrismLinux package repository",
	Long:  `CLI tool to manage PrismLinux package repository. Syncs packages, updates repo DB, generates metadata for web UI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := NewConfig(cmd)
		if err != nil {
			return err
		}
		return runPackageManagement(cfg)
	},
}

func runPackageManagement(cfg *Config) error {
	cfg.debugLog("Starting with config: %+v", cfg)

	// Initialize managers
	pm, err := NewPackageManager(cfg)
	if err != nil {
		return err
	}

	gitMgr := NewGitManager(cfg)
	fileMgr := NewFileManager(cfg)

	if cfg.CleanMode {
		cfg.debugLog("Clean mode enabled")
		return pm.cleanAllFiles(gitMgr, fileMgr)
	}

	// Create necessary directories
	if err := fileMgr.createDirectories(); err != nil {
		return err
	}

	// Setup git operations
	if err := gitMgr.setupGitOperations(); err != nil {
		return err
	}

	// Main package management workflow
	if err := pm.syncPackages(); err != nil {
		return err
	}

	// Finalize git operations
	if err := gitMgr.finalizeGitOperations(); err != nil {
		return err
	}

	fmt.Println("Package management completed successfully.")
	return nil
}

// FileManager methods
func (fm *FileManager) createDirectories() error {
	dirs := []string{fm.config.RepoArchDir, fm.config.APIDir}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (fm *FileManager) removeAllPackages() error {
	fmt.Println("Removing all package files...")

	files, err := os.ReadDir(fm.config.RepoArchDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Repository directory doesn't exist, nothing to clean.")
			return nil
		}
		return fmt.Errorf("failed to read repository directory: %w", err)
	}

	packageCount := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pkg.tar.zst") {
			filePath := filepath.Join(fm.config.RepoArchDir, file.Name())
			fmt.Printf("  -> Removing package: %s\n", file.Name())
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to remove package %s: %w", file.Name(), err)
			}
			packageCount++
		}
	}

	fmt.Printf("Removed %d package files\n", packageCount)
	return nil
}

func (fm *FileManager) removeRepositoryDatabase() error {
	fmt.Println("Removing repository database files...")

	dbFiles := []string{
		fm.config.RepoName + ".db",
		fm.config.RepoName + ".db.tar.gz",
		fm.config.RepoName + ".files",
		fm.config.RepoName + ".files.tar.gz",
	}

	for _, dbFile := range dbFiles {
		filePath := filepath.Join(fm.config.RepoArchDir, dbFile)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("  -> Warning: failed to remove %s: %v\n", dbFile, err)
		} else if err == nil {
			fmt.Printf("  -> Removed database file: %s\n", dbFile)
		}
	}

	return nil
}

func (fm *FileManager) createEmptyPackagesJSON() error {
	fmt.Println("Creating empty packages.json...")

	if err := os.MkdirAll(fm.config.APIDir, 0755); err != nil {
		return fmt.Errorf("failed to create API directory: %w", err)
	}

	emptyPackageList := []PackageInfo{}
	jsonData, err := json.MarshalIndent(emptyPackageList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal empty JSON: %w", err)
	}

	err = os.WriteFile(filepath.Join(fm.config.APIDir, "packages.json"), jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write empty packages.json: %w", err)
	}

	fmt.Println("Created empty packages.json")
	return nil
}

// GitManager methods
func (gm *GitManager) setupGitOperations() error {
	if gm.config.TestMode {
		fmt.Println("Running in test mode - creating test branch...")
		return gm.setupTestBranch()
	} else if gm.config.Commit {
		fmt.Println("Ensuring git repository is initialized...")
		if err := gm.initializeGitRepo(); err != nil {
			return fmt.Errorf("failed to initialize git repository: %w", err)
		}

		fmt.Println("Checking out 'packages' branch...")
		return gm.checkoutOrCreateBranch("packages")
	} else {
		fmt.Println("Skipping git operations (not committing)")
		return nil
	}
}

func (gm *GitManager) finalizeGitOperations() error {
	if gm.config.TestMode {
		fmt.Println("Test mode complete - files saved to test branch.")
		fmt.Println("To clean up: git branch -D test-packages")
		return nil
	} else if gm.config.Commit {
		fmt.Println("Committing and pushing changes to 'packages' branch...")
		return gm.commitAndPushBranch("packages", "Update packages and repository database")
	} else {
		fmt.Println("Skipping commit/push. Use --commit flag to enable or --test for local testing.")
		return nil
	}
}

func (gm *GitManager) setupTestBranch() error {
	if err := gm.ensureGitRepo(); err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	branchName := "test-packages"

	defaultBranch, err := gm.getDefaultBranch()
	if err != nil {
		return fmt.Errorf("failed to determine default branch: %w", err)
	}

	cmd := exec.Command("git", "checkout", defaultBranch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout default branch '%s': %w", defaultBranch, err)
	}

	// Delete existing test branch if it exists
	exec.Command("git", "branch", "-D", branchName).Run()

	cmd = exec.Command("git", "checkout", "-b", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create test branch '%s': %w", branchName, err)
	}

	fmt.Printf("Successfully created test branch '%s'\n", branchName)
	return nil
}

func (gm *GitManager) checkoutOrCreateBranch(branchName string) error {
	if err := gm.ensureGitRepo(); err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	// Try to checkout existing branch
	cmd := exec.Command("git", "checkout", branchName)
	if err := cmd.Run(); err == nil {
		fmt.Printf("Checked out existing branch '%s'\n", branchName)
		return nil
	}

	// Create new branch from default branch
	defaultBranch, err := gm.getDefaultBranch()
	if err != nil {
		return fmt.Errorf("failed to determine default branch: %w", err)
	}

	cmd = exec.Command("git", "checkout", defaultBranch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout default branch '%s': %w", defaultBranch, err)
	}

	cmd = exec.Command("git", "checkout", "-b", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch '%s': %w", branchName, err)
	}

	fmt.Printf("Created and checked out new branch '%s'\n", branchName)
	return nil
}

func (gm *GitManager) ensureGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Stderr = nil
	return cmd.Run()
}

func (gm *GitManager) getDefaultBranch() (string, error) {
	// Try to get the default branch from remote
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if output, err := cmd.Output(); err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Try common branch names
	branches := []string{"main", "master", "develop"}
	for _, branch := range branches {
		cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		if cmd.Run() == nil {
			return branch, nil
		}
	}

	// Fall back to current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if output, err := cmd.Output(); err == nil {
		currentBranch := strings.TrimSpace(string(output))
		if currentBranch != "HEAD" {
			return currentBranch, nil
		}
	}

	return "", fmt.Errorf("could not determine default branch")
}

func (gm *GitManager) initializeGitRepo() error {
	if err := gm.ensureGitRepo(); err == nil {
		return nil
	}

	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Set git configuration
	exec.Command("git", "config", "user.name", "Package Manager").Run()
	exec.Command("git", "config", "user.email", "package-manager@prismlinux.org").Run()

	// Create initial README if it doesn't exist
	if _, err := os.Stat("README.md"); os.IsNotExist(err) {
		readmeContent := "# Package Repository\n\nManaged by package manager tool.\n"
		if err := os.WriteFile("README.md", []byte(readmeContent), 0644); err != nil {
			return fmt.Errorf("failed to create README.md: %w", err)
		}
	}

	cmd = exec.Command("git", "add", "README.md")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add README.md: %w", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	fmt.Println("Git repository initialized")
	return nil
}

func (gm *GitManager) commitAndPushBranch(branchName, message string) error {
	// Set git user configuration for CI
	cmd := exec.Command("git", "config", "user.name", "GitLab CI/Package Manager")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user name: %w", err)
	}

	cmd = exec.Command("git", "config", "user.email", "ci@prismlinux.org")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user email: %w", err)
	}

	// FIXED: Safely remove all tracked files except .git directory
	// Get list of all tracked files
	cmd = exec.Command("git", "ls-files")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list git files: %w", err)
	}

	// Only remove files if there are any tracked files
	trackedFiles := strings.TrimSpace(string(output))
	if trackedFiles != "" {
		files := strings.Split(trackedFiles, "\n")
		// Remove files in batches to avoid command line length limits
		for i := 0; i < len(files); i += 100 {
			end := i + 100
			if end > len(files) {
				end = len(files)
			}
			batch := files[i:end]

			args := append([]string{"rm", "-f", "--ignore-unmatch"}, batch...)
			cmd = exec.Command("git", args...)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to remove tracked files batch: %w", err)
			}
		}
	}

	// Add back only the public/ directory
	cmd = exec.Command("git", "add", "public/")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add public/: %w", err)
	}

	// Commit changes
	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	if err := cmd.Run(); err != nil {
		// Check if it's just a "nothing to commit" message
		if !strings.Contains(err.Error(), "nothing to commit") {
			return fmt.Errorf("failed to commit: %w", err)
		}
	}

	// Push with force-with-lease (safe for artifact branches)
	cmd = exec.Command("git", "push", "origin", branchName, "--force-with-lease")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branchName, err)
	}

	fmt.Printf("✅ Branch '%s' updated with only 'public/' directory\n", branchName)
	return nil
}

// PackageManager methods
func (pm *PackageManager) syncPackages() error {
	// Read remote packages
	remotePackages, err := pm.readRemotePackages()
	if err != nil {
		return fmt.Errorf("failed to read remote package lists: %w", err)
	}

	// Fetch GitLab packages
	gitlabPackages, err := pm.fetchGitLabPackages()
	if err != nil {
		return fmt.Errorf("failed to fetch packages from GitLab releases: %w", err)
	}

	// Combine all remote packages
	allRemotePackages := make(map[string]RemotePackage)
	for _, pkg := range remotePackages {
		allRemotePackages[pkg.Filename] = pkg
	}
	for _, pkg := range gitlabPackages {
		allRemotePackages[pkg.Filename] = pkg
	}

	// Sync packages
	if err := pm.removeOrphanedPackages(allRemotePackages); err != nil {
		return fmt.Errorf("failed to remove orphaned packages: %w", err)
	}

	if err := pm.downloadNewPackages(allRemotePackages); err != nil {
		return fmt.Errorf("failed to download new packages: %w", err)
	}

	if err := pm.updateRepoDatabase(); err != nil {
		return fmt.Errorf("failed to update repository database: %w", err)
	}

	if err := pm.generatePackagesJSON(); err != nil {
		return fmt.Errorf("failed to generate packages.json: %w", err)
	}

	return nil
}

func (pm *PackageManager) cleanAllFiles(gitMgr *GitManager, fileMgr *FileManager) error {
	fmt.Println("Starting cleanup mode - removing all packages and repository files...")

	// Setup git operations for cleanup
	if err := gitMgr.setupGitOperations(); err != nil {
		return err
	}

	// Perform cleanup operations
	if err := fileMgr.removeAllPackages(); err != nil {
		return fmt.Errorf("failed to remove packages: %w", err)
	}

	if err := fileMgr.removeRepositoryDatabase(); err != nil {
		return fmt.Errorf("failed to remove repository database: %w", err)
	}

	if err := fileMgr.createEmptyPackagesJSON(); err != nil {
		return fmt.Errorf("failed to create empty packages.json: %w", err)
	}

	// Finalize git operations
	if pm.config.TestMode {
		fmt.Println("Cleanup test mode complete - files removed from test branch.")
		fmt.Println("Use 'git log --oneline test-packages' to see changes.")
		fmt.Println("To clean up: git branch -D test-packages")
	} else if pm.config.Commit {
		fmt.Println("Committing and pushing cleanup changes...")
		if err := gitMgr.commitAndPushBranch("packages", "Clean up all packages and repository files"); err != nil {
			return fmt.Errorf("failed to commit and push cleanup: %w", err)
		}
	} else {
		fmt.Println("Cleanup complete. Use --commit to save changes or --test for test mode.")
	}

	fmt.Println("All packages and repository files have been removed successfully.")
	return nil
}

func (pm *PackageManager) readRemotePackages() ([]RemotePackage, error) {
	var packages []RemotePackage

	file, err := os.Open("remote_packages.txt")
	if os.IsNotExist(err) {
		pm.config.infoLog("remote_packages.txt not found. No remote HTTPS packages to process.")
		return packages, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to open remote_packages.txt: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url == "" || strings.HasPrefix(url, "#") {
			continue
		}
		if strings.HasSuffix(url, ".pkg.tar.zst") {
			filename := filepath.Base(strings.Split(url, "?")[0])
			if filename != "" {
				packages = append(packages, RemotePackage{Filename: filename, URL: url})
			}
		}
	}

	return packages, scanner.Err()
}

func (pm *PackageManager) fetchGitLabPackages() ([]RemotePackage, error) {
	var packages []RemotePackage

	if pm.gitlabClient == nil {
		return packages, nil
	}

	projectIDs, err := pm.readProjectIDs("packages_id.txt")
	if err != nil {
		return nil, err
	}

	for _, projectID := range projectIDs {
		listOptions := &gitlab.ListReleasesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 1},
		}

		releases, _, err := pm.gitlabClient.Releases.ListReleases(projectID, listOptions)
		if err != nil {
			fmt.Printf("[WARNING] Failed to fetch releases for project %s: %v. Skipping.\n", projectID, err)
			continue
		}
		if len(releases) == 0 {
			continue
		}
		latestRelease := releases[0]

		for _, asset := range latestRelease.Assets.Links {
			if strings.HasSuffix(asset.Name, ".pkg.tar.zst") && strings.HasPrefix(asset.URL, "https://") {
				packages = append(packages, RemotePackage{Filename: asset.Name, URL: asset.URL})
			}
		}
	}

	return packages, nil
}

func (pm *PackageManager) readProjectIDs(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("packages_id.txt not found: %w", err)
	}
	defer file.Close()

	var ids []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) > 0 {
			ids = append(ids, parts[0])
		}
	}
	return ids, scanner.Err()
}

func (pm *PackageManager) removeOrphanedPackages(remotePackages map[string]RemotePackage) error {
	files, err := os.ReadDir(pm.config.RepoArchDir)
	if err != nil {
		return fmt.Errorf("failed to read local directory: %w", err)
	}

	removedCount := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pkg.tar.zst") {
			if _, exists := remotePackages[file.Name()]; !exists {
				filePath := filepath.Join(pm.config.RepoArchDir, file.Name())
				pm.config.verboseLog("Removing orphaned package: %s", file.Name())
				if err := os.Remove(filePath); err != nil {
					return fmt.Errorf("failed to remove orphaned package %s: %w", file.Name(), err)
				}
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		pm.config.infoLog("Removed %d orphaned packages", removedCount)
	}

	return nil
}

func (pm *PackageManager) downloadNewPackages(remotePackages map[string]RemotePackage) error {
	downloadedCount := 0
	for filename, pkg := range remotePackages {
		localPath := filepath.Join(pm.config.RepoArchDir, filename)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			pm.config.verboseLog("Downloading package: %s", filename)
			if err := pm.downloadFile(localPath, pkg.URL); err != nil {
				pm.config.debugLog("Failed to download %s: %v", filename, err)
				os.Remove(localPath) // Clean up partial download
				continue
			}
			downloadedCount++
		}
	}

	if downloadedCount > 0 {
		pm.config.infoLog("Downloaded %d new packages", downloadedCount)
	}

	return nil
}

func (pm *PackageManager) downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filepath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", filepath, err)
	}

	return nil
}

func (pm *PackageManager) updateRepoDatabase() error {
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(pm.config.RepoArchDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	// Remove existing database files
	dbFiles := []string{
		pm.config.RepoName + ".db",
		pm.config.RepoName + ".db.tar.gz",
		pm.config.RepoName + ".files",
		pm.config.RepoName + ".files.tar.gz",
	}

	for _, file := range dbFiles {
		os.Remove(file)
	}

	// Check for package files
	matches, err := filepath.Glob("*.pkg.tar.zst")
	if err != nil {
		return fmt.Errorf("failed to check for packages: %w", err)
	}

	if len(matches) > 0 {
		args := append([]string{pm.config.RepoName + ".db.tar.gz"}, matches...)
		cmd := exec.Command("repo-add", args...)
		if pm.config.Debug {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run repo-add: %w", err)
		}
		pm.config.infoLog("Updated repository database with %d packages", len(matches))
	} else {
		// Create empty database files
		os.WriteFile(pm.config.RepoName+".db.tar.gz", []byte{}, 0644)
		os.WriteFile(pm.config.RepoName+".files.tar.gz", []byte{}, 0644)
		pm.config.infoLog("Created empty repository database")
	}

	// Create symlinks
	os.Remove(pm.config.RepoName + ".db")
	os.Remove(pm.config.RepoName + ".files")
	os.Symlink(pm.config.RepoName+".db.tar.gz", pm.config.RepoName+".db")
	os.Symlink(pm.config.RepoName+".files.tar.gz", pm.config.RepoName+".files")

	return nil
}

func (pm *PackageManager) generatePackagesJSON() error {
	var packageList []PackageInfo

	files, err := os.ReadDir(pm.config.RepoArchDir)
	if err != nil {
		return fmt.Errorf("failed to read repository directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pkg.tar.zst") {
			pkgPath := filepath.Join(pm.config.RepoArchDir, file.Name())
			pkgInfo, err := pm.extractPackageInfo(pkgPath)
			if err != nil {
				pm.config.debugLog("Failed to extract package info for %s: %v", file.Name(), err)
				continue
			}
			packageList = append(packageList, *pkgInfo)
		}
	}

	jsonData, err := json.MarshalIndent(packageList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	outputPath := filepath.Join(pm.config.APIDir, "packages.json")
	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write packages.json: %w", err)
	}

	pm.config.infoLog("Generated packages.json with %d packages", len(packageList))
	return nil
}

func (pm *PackageManager) extractPackageInfo(pkgPath string) (*PackageInfo, error) {
	cmd := exec.Command("pacman", "-Qip", pkgPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pacman -Qip failed: %w", err)
	}

	info := &PackageInfo{
		Depends: "None",
		Groups:  "None",
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Name":
			info.Name = value
		case "Version":
			info.Version = value
		case "Description":
			info.Description = value
		case "Architecture":
			info.Architecture = value
		case "Depends On":
			info.Depends = value
		case "Groups":
			info.Groups = value
		}
	}

	info.Filename = filepath.Base(pkgPath)
	fileInfo, err := os.Stat(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("could not stat package file: %w", err)
	}
	info.Size = fmt.Sprintf("%d", fileInfo.Size())
	info.Modified = fileInfo.ModTime().Format("2006-01-02 15:04:05")

	return info, nil
}

func init() {
	RootCmd.Flags().String("repo-name", "prismlinux", "Repository name")
	RootCmd.Flags().String("repo-arch-dir", "public/x86_64", "Architecture-specific repo directory")
	RootCmd.Flags().String("api-dir", "public/api", "API directory for metadata")
	RootCmd.Flags().String("gitlab-token", "", "GitLab token (overrides GITLAB_TOKEN env)")
	RootCmd.Flags().String("project-id", "", "GitLab project ID (overrides CI_PROJECT_ID env)")
	RootCmd.Flags().Bool("commit", false, "Commit and push to 'packages' branch")
	RootCmd.Flags().Bool("test", false, "Test mode - use test-packages branch, no push")
	RootCmd.Flags().Bool("clean", false, "Clean mode - remove all packages and repo files")
	RootCmd.Flags().Bool("debug", false, "Enable debug output")
	RootCmd.Flags().Bool("verbose", false, "Enable verbose output")
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
