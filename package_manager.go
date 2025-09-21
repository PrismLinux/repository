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
	"gopkg.in/yaml.v3"
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
	Repository   string `json:"repository"`
}

type RemotePackage struct {
	Filename   string
	URL        string
	Repository string
}

type GitLabProject struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	Repository string `yaml:"repository"`
	Enabled    bool   `yaml:"enabled"`
}

type RemoteURL struct {
	URL        string `yaml:"url"`
	Repository string `yaml:"repository"`
	Enabled    bool   `yaml:"enabled"`
}

type PackagesConfig struct {
	GitLabProjects []GitLabProject `yaml:"gitlab_projects"`
	RemoteURLs     []RemoteURL     `yaml:"remote_urls"`
}

type Config struct {
	RepoName    string
	RepoArchDir string
	APIDir      string
	GitLabToken string
	TestingMode bool
	Debug       bool
	Verbose     bool
}

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

type PackageManager struct {
	config       *Config
	gitlabClient *gitlab.Client
}

func NewPackageManager(cfg *Config) (*PackageManager, error) {
	pm := &PackageManager{config: cfg}

	if cfg.GitLabToken != "" {
		git, err := gitlab.NewClient(cfg.GitLabToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitLab client: %w", err)
		}
		pm.gitlabClient = git
		cfg.debugLog("GitLab client initialized successfully")
	} else {
		cfg.debugLog("No GitLab token provided - will only process remote URLs")
	}

	return pm, nil
}

type FileManager struct {
	config *Config
}

func NewFileManager(cfg *Config) *FileManager {
	return &FileManager{config: cfg}
}

func getStringFlag(cmd *cobra.Command, name, defaultValue string) string {
	if value, _ := cmd.Flags().GetString(name); value != "" {
		return value
	}
	return defaultValue
}

func NewConfig(cmd *cobra.Command) (*Config, error) {
	cfg := &Config{}

	cfg.RepoName = getStringFlag(cmd, "repo-name", "prismlinux")
	cfg.TestingMode, _ = cmd.Flags().GetBool("testing")

	architecture := getStringFlag(cmd, "arch", "x86_64")

	if cfg.TestingMode {
		cfg.RepoArchDir = getStringFlag(cmd, "repo-arch-dir", filepath.Join("testing", architecture))
	} else {
		cfg.RepoArchDir = getStringFlag(cmd, "repo-arch-dir", architecture)
	}

	cfg.APIDir = getStringFlag(cmd, "api-dir", "api")
	cfg.GitLabToken = getStringFlag(cmd, "gitlab-token", "")
	if cfg.GitLabToken == "" {
		cfg.GitLabToken = os.Getenv("GITLAB_TOKEN")
	}

	cfg.Debug, _ = cmd.Flags().GetBool("debug")
	cfg.Verbose, _ = cmd.Flags().GetBool("verbose")

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

func readPackagesConfig() (*PackagesConfig, error) {
	configFile := "packages_config.yaml"

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		defaultConfig := &PackagesConfig{
			GitLabProjects: []GitLabProject{
				{
					ID:         "12345",
					Name:       "example-package",
					Repository: "stable",
					Enabled:    true,
				},
			},
			RemoteURLs: []RemoteURL{
				{
					URL:        "https://example.com/package.pkg.tar.zst  ",
					Repository: "stable",
					Enabled:    true,
				},
			},
		}

		data, err := yaml.Marshal(defaultConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default config: %w", err)
		}

		if err := os.WriteFile(configFile, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}

		fmt.Printf("Created default config file: %s\n", configFile)
		fmt.Println("Please edit the config file and run the command again.")
		return defaultConfig, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config PackagesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func runPackageManagement(cfg *Config) error {
	cfg.debugLog("Starting with config: %+v", cfg)

	packagesConfig, err := readPackagesConfig()
	if err != nil {
		return fmt.Errorf("failed to read packages configuration: %w", err)
	}

	pm, err := NewPackageManager(cfg)
	if err != nil {
		return err
	}

	fileMgr := NewFileManager(cfg)

	if err := fileMgr.createDirectories(); err != nil {
		return err
	}

	if err := pm.syncPackages(packagesConfig); err != nil {
		return err
	}

	fmt.Println("Package management completed successfully.")
	return nil
}

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
	fmt.Println("Creating empty API files...")

	if err := os.MkdirAll(fm.config.APIDir, 0755); err != nil {
		return fmt.Errorf("failed to create API directory: %w", err)
	}

	emptyPackageList := []PackageInfo{}
	jsonData, err := json.MarshalIndent(emptyPackageList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal empty JSON: %w", err)
	}

	targetRepo := "stable"
	if fm.config.TestingMode {
		targetRepo = "testing"
	}

	apiFileName := fmt.Sprintf("%s.json", targetRepo)
	apiFilePath := filepath.Join(fm.config.APIDir, apiFileName)

	err = os.WriteFile(apiFilePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", apiFileName, err)
	}

	fmt.Printf("Created empty %s\n", apiFileName)
	return nil
}

func (pm *PackageManager) syncPackages(packagesConfig *PackagesConfig) error {
	targetRepo := "stable"
	if pm.config.TestingMode {
		targetRepo = "testing"
	}

	pm.config.infoLog("Syncing packages for repository: %s", targetRepo)

	gitlabPackages, err := pm.fetchGitLabPackages(packagesConfig.GitLabProjects, targetRepo)
	if err != nil {
		return fmt.Errorf("failed to fetch packages from GitLab releases: %w", err)
	}

	remotePackages, err := pm.fetchRemoteURLPackages(packagesConfig.RemoteURLs, targetRepo)
	if err != nil {
		return fmt.Errorf("failed to fetch remote URL packages: %w", err)
	}

	allRemotePackages := make(map[string]RemotePackage)
	for _, pkg := range gitlabPackages {
		allRemotePackages[pkg.Filename] = pkg
	}
	for _, pkg := range remotePackages {
		allRemotePackages[pkg.Filename] = pkg
	}

	pm.config.infoLog("Found %d packages total", len(allRemotePackages))

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

func (pm *PackageManager) fetchGitLabPackages(projects []GitLabProject, targetRepo string) ([]RemotePackage, error) {
	var packages []RemotePackage

	if pm.gitlabClient == nil {
		pm.config.infoLog("No GitLab client available, skipping GitLab packages")
		return packages, nil
	}

	enabledProjects := make([]GitLabProject, 0)
	for _, project := range projects {
		if project.Enabled {
			enabledProjects = append(enabledProjects, project)
		}
	}

	if len(enabledProjects) == 0 {
		pm.config.infoLog("No enabled GitLab projects found")
		return packages, nil
	}

	// Process each project to determine which packages belong to which repository
	for _, project := range enabledProjects {
		pm.config.verboseLog("Fetching releases for project: %s (%s)", project.Name, project.ID)

		listOptions := &gitlab.ListReleasesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 10},
		}

		releases, _, err := pm.gitlabClient.Releases.ListReleases(project.ID, listOptions)
		if err != nil {
			fmt.Printf("[WARNING] Failed to fetch releases for project %s (%s): %v. Skipping.\n", project.Name, project.ID, err)
			continue
		}
		if len(releases) == 0 {
			pm.config.verboseLog("No releases found for project: %s", project.Name)
			continue
		}

		// Check if this project should be included in the target repository
		if project.Repository == targetRepo {
			pm.config.verboseLog("Found release: %s for project %s", releases[0].Name, project.Name)

			for _, asset := range releases[0].Assets.Links {
				if strings.HasSuffix(asset.Name, ".pkg.tar.zst") && strings.HasPrefix(asset.URL, "https") {
					packages = append(packages, RemotePackage{
						Filename:   asset.Name,
						URL:        asset.URL,
						Repository: project.Repository,
					})
					pm.config.verboseLog("Added package: %s from project %s", asset.Name, project.Name)
				}
			}
		}
	}

	pm.config.infoLog("Found %d packages from GitLab projects", len(packages))
	return packages, nil
}

func (pm *PackageManager) fetchRemoteURLPackages(remoteURLs []RemoteURL, targetRepo string) ([]RemotePackage, error) {
	var packages []RemotePackage

	for _, remote := range remoteURLs {
		if !remote.Enabled || remote.Repository != targetRepo {
			continue
		}

		cleanURL := strings.TrimSpace(remote.URL)
		if strings.HasSuffix(cleanURL, ".pkg.tar.zst") {
			filename := filepath.Base(strings.Split(cleanURL, "?")[0])
			if filename != "" {
				packages = append(packages, RemotePackage{
					Filename:   filename,
					URL:        cleanURL,
					Repository: remote.Repository,
				})
				pm.config.verboseLog("Added remote package: %s", filename)
			}
		}
	}

	pm.config.infoLog("Found %d packages from remote URLs", len(packages))
	return packages, nil
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
			pm.config.verboseLog("Downloading package: %s from %s", filename, pkg.URL)
			if err := pm.downloadFile(localPath, pkg.URL); err != nil {
				pm.config.debugLog("Failed to download %s: %v", filename, err)
				os.Remove(localPath)
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

	if err := os.Chdir(pm.config.RepoArchDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", pm.config.RepoArchDir, err)
	}

	defer func() {
		if chdirErr := os.Chdir(originalDir); chdirErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to change back to original directory %s: %v\n", originalDir, chdirErr)
			os.Exit(1)
		}
	}()

	dbFiles := []string{
		pm.config.RepoName + ".db",
		pm.config.RepoName + ".db.tar.gz",
		pm.config.RepoName + ".files",
		pm.config.RepoName + ".files.tar.gz",
	}

	for _, file := range dbFiles {
		os.Remove(file)
	}

	matches, err := filepath.Glob("*.pkg.tar.zst")
	if err != nil {
		return fmt.Errorf("failed to check for packages: %w", err)
	}

	if len(matches) > 0 {
		args := append([]string{pm.config.RepoName + ".db.tar.gz"}, matches...)
		cmd := exec.Command("repo-add", args...)
		if pm.config.Debug || pm.config.Verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run repo-add: %w", err)
		}
		pm.config.infoLog("Updated repository database with %d packages", len(matches))
	} else {
		os.WriteFile(pm.config.RepoName+".db.tar.gz", []byte{}, 0644)
		os.WriteFile(pm.config.RepoName+".files.tar.gz", []byte{}, 0644)
		pm.config.infoLog("Created empty repository database")
	}

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

	targetRepo := "stable"
	if pm.config.TestingMode {
		targetRepo = "testing"
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pkg.tar.zst") {
			pkgPath := filepath.Join(pm.config.RepoArchDir, file.Name())
			pkgInfo, err := pm.extractPackageInfo(pkgPath)
			if err != nil {
				pm.config.debugLog("Failed to extract package info for %s: %v", file.Name(), err)
				continue
			}
			pkgInfo.Repository = targetRepo
			packageList = append(packageList, *pkgInfo)
		}
	}

	jsonData, err := json.MarshalIndent(packageList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	apiFileName := fmt.Sprintf("%s.json", targetRepo)
	outputPath := filepath.Join(pm.config.APIDir, apiFileName)

	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", apiFileName, err)
	}

	pm.config.infoLog("Generated %s with %d packages", apiFileName, len(packageList))
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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show repository structure and current status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := NewConfig(cmd)
		if err != nil {
			return err
		}

		return showRepositoryStatus(cfg)
	},
}

func showRepositoryStatus(cfg *Config) error {
	fmt.Println("=== PrismLinux Repository Structure ===")
	fmt.Println()

	targetRepo := "stable"
	if cfg.TestingMode {
		targetRepo = "testing"
	}

	fmt.Printf("Current mode: %s repository\n", targetRepo)
	fmt.Printf("Architecture directory: %s\n", cfg.RepoArchDir)
	fmt.Printf("API directory: %s\n", cfg.APIDir)
	fmt.Println()

	if _, err := os.Stat(cfg.RepoArchDir); err == nil {
		files, err := os.ReadDir(cfg.RepoArchDir)
		if err != nil {
			return fmt.Errorf("failed to read repository directory: %w", err)
		}

		packageCount := 0
		fmt.Printf("=== Packages in %s repository ===\n", targetRepo)
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".pkg.tar.zst") {
				info, _ := file.Info()
				fmt.Printf("  %s (%s)\n", file.Name(), formatSize(info.Size()))
				packageCount++
			}
		}

		if packageCount == 0 {
			fmt.Println("  No packages found")
		} else {
			fmt.Printf("  Total: %d packages\n", packageCount)
		}
	} else {
		fmt.Printf("Repository directory does not exist: %s\n", cfg.RepoArchDir)
	}
	fmt.Println()

	fmt.Println("=== API Files ===")
	apiFiles := []string{"stable.json", "testing.json"}
	for _, apiFile := range apiFiles {
		apiPath := filepath.Join(cfg.APIDir, apiFile)
		if info, err := os.Stat(apiPath); err == nil {
			fmt.Printf("  %s (%s)\n", apiFile, formatSize(info.Size()))
		} else {
			fmt.Printf("  %s (not found)\n", apiFile)
		}
	}
	fmt.Println()

	if info, err := os.Stat("packages_config.yaml"); err == nil {
		fmt.Printf("Configuration file: packages_config.yaml (%s)\n", formatSize(info.Size()))
	} else {
		fmt.Println("Configuration file: packages_config.yaml (not found)")
	}

	return nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all packages and repository files",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := NewConfig(cmd)
		if err != nil {
			return err
		}

		fileMgr := NewFileManager(cfg)

		repoType := "stable"
		if cfg.TestingMode {
			repoType = "testing"
		}
		fmt.Printf("Starting cleanup mode for %s repository...\n", repoType)

		if err := fileMgr.removeAllPackages(); err != nil {
			return fmt.Errorf("failed to remove packages: %w", err)
		}

		if err := fileMgr.removeRepositoryDatabase(); err != nil {
			return fmt.Errorf("failed to remove repository database: %w", err)
		}

		if err := fileMgr.createEmptyPackagesJSON(); err != nil {
			return fmt.Errorf("failed to create empty packages.json: %w", err)
		}

		fmt.Println("All packages and repository files have been removed successfully.")
		return nil
	},
}

func init() {
	RootCmd.Flags().String("repo-name", "prismlinux", "Repository name")
	RootCmd.Flags().String("arch", "x86_64", "Target architecture")
	RootCmd.Flags().String("repo-arch-dir", "", "Architecture-specific repo directory (auto-determined)")
	RootCmd.Flags().String("api-dir", "api", "API directory for metadata")
	RootCmd.Flags().String("gitlab-token", "", "GitLab token (overrides GITLAB_TOKEN env)")
	RootCmd.Flags().Bool("testing", false, "Use testing repository instead of stable")
	RootCmd.Flags().Bool("debug", false, "Enable debug output")
	RootCmd.Flags().Bool("verbose", false, "Enable verbose output")

	RootCmd.AddCommand(cleanCmd)
	RootCmd.AddCommand(statusCmd)

	cleanCmd.Flags().String("repo-name", "prismlinux", "Repository name")
	cleanCmd.Flags().String("arch", "x86_64", "Target architecture")
	cleanCmd.Flags().String("repo-arch-dir", "", "Architecture-specific repo directory (auto-determined)")
	cleanCmd.Flags().String("api-dir", "api", "API directory for metadata")
	cleanCmd.Flags().Bool("testing", false, "Clean testing repository instead of stable")
	cleanCmd.Flags().Bool("debug", false, "Enable debug output")
	cleanCmd.Flags().Bool("verbose", false, "Enable verbose output")

	statusCmd.Flags().String("repo-name", "prismlinux", "Repository name")
	statusCmd.Flags().String("arch", "x86_64", "Target architecture")
	statusCmd.Flags().String("repo-arch-dir", "", "Architecture-specific repo directory (auto-determined)")
	statusCmd.Flags().String("api-dir", "api", "API directory for metadata")
	statusCmd.Flags().Bool("testing", false, "Show testing repository status")
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
