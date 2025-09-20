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

func NewConfig(cmd *cobra.Command) (*Config, error) {
	repoName, _ := cmd.Flags().GetString("repo-name")
	if repoName == "" {
		repoName = "prismlinux"
	}

	repoArchDir, _ := cmd.Flags().GetString("repo-arch-dir")
	if repoArchDir == "" {
		repoArchDir = "public/x86_64"
	}

	apiDir, _ := cmd.Flags().GetString("api-dir")
	if apiDir == "" {
		apiDir = "public/api"
	}

	gitLabToken, _ := cmd.Flags().GetString("gitlab-token")
	if gitLabToken == "" {
		gitLabToken = os.Getenv("GITLAB_TOKEN")
	}

	currentProjID, _ := cmd.Flags().GetString("project-id")
	if currentProjID == "" {
		currentProjID = os.Getenv("CI_PROJECT_ID")
	}

	commit, _ := cmd.Flags().GetBool("commit")
	testMode, _ := cmd.Flags().GetBool("test")
	cleanMode, _ := cmd.Flags().GetBool("clean")
	debug, _ := cmd.Flags().GetBool("debug")
	verbose, _ := cmd.Flags().GetBool("verbose")

	if gitLabToken == "" && !cleanMode {
		return nil, fmt.Errorf("GITLAB_TOKEN is required (set via flag or environment variable)")
	}

	return &Config{
		RepoName:      repoName,
		RepoArchDir:   repoArchDir,
		APIDir:        apiDir,
		GitLabToken:   gitLabToken,
		CurrentProjID: currentProjID,
		Commit:        commit,
		TestMode:      testMode,
		CleanMode:     cleanMode,
		Debug:         debug,
		Verbose:       verbose,
	}, nil
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
	if cfg.Debug {
		fmt.Printf("[DEBUG] Starting with config: %+v\n", cfg)
	}

	if cfg.CleanMode {
		if cfg.Debug {
			fmt.Println("[DEBUG] Clean mode enabled")
		}
		return cleanAllFiles(cfg)
	}

	if err := os.MkdirAll(cfg.RepoArchDir, 0755); err != nil {
		return fmt.Errorf("failed to create repo arch directory: %w", err)
	}
	if err := os.MkdirAll(cfg.APIDir, 0755); err != nil {
		return fmt.Errorf("failed to create api directory: %w", err)
	}

	git, err := gitlab.NewClient(cfg.GitLabToken)
	if err != nil {
		return fmt.Errorf("failed to create GitLab client: %w", err)
	}

	if cfg.TestMode {
		fmt.Println("Running in test mode - creating test branch...")
		if err := setupTestBranch(); err != nil {
			return fmt.Errorf("failed to setup test branch: %w", err)
		}
	} else if cfg.Commit {
		fmt.Println("Ensuring git repository is initialized...")
		if err := initializeGitRepo(); err != nil {
			return fmt.Errorf("failed to initialize git repository: %w", err)
		}

		fmt.Println("Checking out 'packages' branch...")
		if err := checkoutOrCreateBranch("packages"); err != nil {
			return fmt.Errorf("failed to checkout/create 'packages' branch: %w", err)
		}
	} else {
		fmt.Println("Skipping git operations (not committing)")
	}

	remotePackages, err := readRemotePackages(cfg)
	if err != nil {
		return fmt.Errorf("failed to read remote package lists: %w", err)
	}

	gitlabPackages, err := fetchGitLabPackages(git, cfg.CurrentProjID, cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch packages from GitLab releases: %w", err)
	}

	allRemotePackages := make(map[string]RemotePackage)
	for _, pkg := range remotePackages {
		allRemotePackages[pkg.Filename] = pkg
	}
	for _, pkg := range gitlabPackages {
		allRemotePackages[pkg.Filename] = pkg
	}

	if err := removeOrphanedPackages(cfg.RepoArchDir, allRemotePackages, cfg); err != nil {
		return fmt.Errorf("failed to remove orphaned packages: %w", err)
	}

	if err := downloadNewPackages(cfg.RepoArchDir, allRemotePackages, cfg); err != nil {
		return fmt.Errorf("failed to download new packages: %w", err)
	}

	if err := updateRepoDatabase(cfg.RepoArchDir, cfg.RepoName, cfg); err != nil {
		return fmt.Errorf("failed to update repository database: %w", err)
	}

	if err := generatePackagesJSON(cfg.RepoArchDir, cfg.APIDir, cfg); err != nil {
		return fmt.Errorf("failed to generate packages.json: %w", err)
	}

	if cfg.TestMode {
		fmt.Println("Test mode complete - files saved to test branch.")
		fmt.Println("To clean up: git branch -D test-packages")
	} else if cfg.Commit {
		fmt.Println("Committing and pushing changes to 'packages' branch...")
		if err := commitAndPushBranch("packages", "Update packages and repository database"); err != nil {
			return fmt.Errorf("failed to commit and push 'packages' branch: %w", err)
		}
	} else {
		fmt.Println("Skipping commit/push. Use --commit flag to enable or --test for local testing.")
	}

	fmt.Println("Package management completed successfully.")
	return nil
}

func debugLog(cfg *Config, format string, args ...interface{}) {
	if cfg.Debug {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

func verboseLog(cfg *Config, format string, args ...interface{}) {
	if cfg.Verbose || cfg.Debug {
		fmt.Printf("[VERBOSE] "+format+"\n", args...)
	}
}

func removeAllPackages(repoDir string) error {
	fmt.Println("Removing all package files...")

	files, err := os.ReadDir(repoDir)
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
			filePath := filepath.Join(repoDir, file.Name())
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

func removeRepositoryDatabase(repoDir, repoName string) error {
	fmt.Println("Removing repository database files...")

	dbFiles := []string{
		repoName + ".db",
		repoName + ".db.tar.gz",
		repoName + ".files",
		repoName + ".files.tar.gz",
	}

	for _, dbFile := range dbFiles {
		filePath := filepath.Join(repoDir, dbFile)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("  -> Warning: failed to remove %s: %v\n", dbFile, err)
		} else if err == nil {
			fmt.Printf("  -> Removed database file: %s\n", dbFile)
		}
	}

	return nil
}

func createEmptyPackagesJSON(apiDir string) error {
	fmt.Println("Creating empty packages.json...")

	if err := os.MkdirAll(apiDir, 0755); err != nil {
		return fmt.Errorf("failed to create API directory: %w", err)
	}

	emptyPackageList := []PackageInfo{}
	jsonData, err := json.MarshalIndent(emptyPackageList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal empty JSON: %w", err)
	}

	err = os.WriteFile(filepath.Join(apiDir, "packages.json"), jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write empty packages.json: %w", err)
	}

	fmt.Println("Created empty packages.json")
	return nil
}

func cleanAllFiles(cfg *Config) error {
	fmt.Println("Starting cleanup mode - removing all packages and repository files...")

	if cfg.TestMode {
		fmt.Println("Setting up test branch for cleanup...")
		if err := setupTestBranch(); err != nil {
			return fmt.Errorf("failed to setup test branch: %w", err)
		}
	} else if cfg.Commit {
		fmt.Println("Ensuring git repository is initialized...")
		if err := initializeGitRepo(); err != nil {
			return fmt.Errorf("failed to initialize git repository: %w", err)
		}

		fmt.Println("Checking out 'packages' branch...")
		if err := checkoutOrCreateBranch("packages"); err != nil {
			return fmt.Errorf("failed to checkout/create 'packages' branch: %w", err)
		}
	}

	if err := removeAllPackages(cfg.RepoArchDir); err != nil {
		return fmt.Errorf("failed to remove packages: %w", err)
	}

	if err := removeRepositoryDatabase(cfg.RepoArchDir, cfg.RepoName); err != nil {
		return fmt.Errorf("failed to remove repository database: %w", err)
	}

	if err := createEmptyPackagesJSON(cfg.APIDir); err != nil {
		return fmt.Errorf("failed to create empty packages.json: %w", err)
	}

	if cfg.TestMode {
		fmt.Println("Cleanup test mode complete - files removed from test branch.")
		fmt.Println("Use 'git log --oneline test-packages' to see changes.")
		fmt.Println("To clean up: git branch -D test-packages")
	} else if cfg.Commit {
		fmt.Println("Committing and pushing cleanup changes...")
		if err := commitAndPushBranch("packages", "Clean up all packages and repository files"); err != nil {
			return fmt.Errorf("failed to commit and push cleanup: %w", err)
		}
	} else {
		fmt.Println("Cleanup complete. Use --commit to save changes or --test for test mode.")
	}

	fmt.Println("All packages and repository files have been removed successfully.")
	return nil
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

func setupTestBranch() error {
	if err := ensureGitRepo(); err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	branchName := "test-packages"

	defaultBranch, err := getDefaultBranch()
	if err != nil {
		return fmt.Errorf("failed to determine default branch: %w", err)
	}

	cmd := exec.Command("git", "checkout", defaultBranch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout default branch '%s': %w", defaultBranch, err)
	}

	exec.Command("git", "branch", "-D", branchName).Run()

	cmd = exec.Command("git", "checkout", "-b", branchName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create test branch '%s': %w", branchName, err)
	}

	fmt.Printf("Successfully created test branch '%s'\n", branchName)
	return nil
}

func checkoutOrCreateBranch(branchName string) error {
	if err := ensureGitRepo(); err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	cmd := exec.Command("git", "checkout", branchName)
	if err := cmd.Run(); err == nil {
		fmt.Printf("Checked out existing branch '%s'\n", branchName)
		return nil
	}

	defaultBranch, err := getDefaultBranch()
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

func ensureGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Stderr = nil
	return cmd.Run()
}

func getDefaultBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if output, err := cmd.Output(); err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	branches := []string{"main", "master", "develop"}
	for _, branch := range branches {
		cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
		if cmd.Run() == nil {
			return branch, nil
		}
	}

	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if output, err := cmd.Output(); err == nil {
		currentBranch := strings.TrimSpace(string(output))
		if currentBranch != "HEAD" {
			return currentBranch, nil
		}
	}

	return "", fmt.Errorf("could not determine default branch")
}

func initializeGitRepo() error {
	if err := ensureGitRepo(); err == nil {
		return nil
	}

	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	exec.Command("git", "config", "user.name", "Package Manager").Run()
	exec.Command("git", "config", "user.email", "package-manager@prismlinux.org").Run()

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

func readRemotePackages(cfg *Config) ([]RemotePackage, error) {
	var packages []RemotePackage

	file, err := os.Open("remote_packages.txt")
	if os.IsNotExist(err) {
		fmt.Println("[INFO] remote_packages.txt not found. No remote HTTPS packages to process.")
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

func fetchGitLabPackages(git *gitlab.Client, currentProjectID string, cfg *Config) ([]RemotePackage, error) {
	var packages []RemotePackage

	projectIDs, err := readProjectIDs("packages_id.txt", cfg)
	if err != nil {
		return nil, err
	}

	for _, projectID := range projectIDs {
		listOptions := &gitlab.ListReleasesOptions{
			ListOptions: gitlab.ListOptions{PerPage: 1},
		}

		releases, _, err := git.Releases.ListReleases(projectID, listOptions)
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

func readProjectIDs(filename string, cfg *Config) ([]string, error) {
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

func removeOrphanedPackages(localDir string, remotePackages map[string]RemotePackage, cfg *Config) error {
	files, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("failed to read local directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pkg.tar.zst") {
			if _, exists := remotePackages[file.Name()]; !exists {
				err := os.Remove(filepath.Join(localDir, file.Name()))
				if err != nil {
					return fmt.Errorf("failed to remove orphaned package %s: %w", file.Name(), err)
				}
			}
		}
	}
	return nil
}

func downloadNewPackages(localDir string, remotePackages map[string]RemotePackage, cfg *Config) error {
	for filename, pkg := range remotePackages {
		localPath := filepath.Join(localDir, filename)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			err := downloadFile(localPath, pkg.URL, cfg)
			if err != nil {
				os.Remove(localPath)
			}
		}
	}
	return nil
}

func downloadFile(filepath string, url string, cfg *Config) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func updateRepoDatabase(repoDir, repoName string, cfg *Config) error {
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	dbFiles := []string{
		repoName + ".db",
		repoName + ".db.tar.gz",
		repoName + ".files",
		repoName + ".files.tar.gz",
	}

	for _, file := range dbFiles {
		os.Remove(file)
	}

	matches, err := filepath.Glob("*.pkg.tar.zst")
	if err != nil {
		return fmt.Errorf("failed to check for packages: %w", err)
	}

	if len(matches) > 0 {
		args := append([]string{repoName + ".db.tar.gz"}, matches...)
		cmd := exec.Command("repo-add", args...)
		if cfg.Debug {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run repo-add: %w", err)
		}
	} else {
		os.WriteFile(repoName+".db.tar.gz", []byte{}, 0644)
		os.WriteFile(repoName+".files.tar.gz", []byte{}, 0644)
	}

	os.Remove(repoName + ".db")
	os.Remove(repoName + ".files")
	os.Symlink(repoName+".db.tar.gz", repoName+".db")
	os.Symlink(repoName+".files.tar.gz", repoName+".files")

	return nil
}

func generatePackagesJSON(repoDir, apiDir string, cfg *Config) error {
	var packageList []PackageInfo

	files, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("failed to read repository directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pkg.tar.zst") {
			pkgPath := filepath.Join(repoDir, file.Name())
			pkgInfo, err := extractPackageInfo(pkgPath, cfg)
			if err != nil {
				continue
			}
			packageList = append(packageList, *pkgInfo)
		}
	}

	jsonData, err := json.MarshalIndent(packageList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	outputPath := filepath.Join(apiDir, "packages.json")
	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write packages.json: %w", err)
	}

	fmt.Printf("[INFO] Generated packages.json with %d packages\n", len(packageList))
	return nil
}

func extractPackageInfo(pkgPath string, cfg *Config) (*PackageInfo, error) {
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

func commitAndPushBranch(branchName, message string) error {
	cmd := exec.Command("git", "config", "user.name", "GitLab CI/Package Manager")
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "config", "user.email", "ci@prismlinux.org")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Remove ALL files except .git
	cmd = exec.Command("git", "rm", "-rf", "--ignore-unmatch", ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove all files: %w", err)
	}

	// Add back only the public/ directory
	cmd = exec.Command("git", "add", "public/")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add public/: %w", err)
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	if err := cmd.Run(); err != nil {
		if !strings.Contains(err.Error(), "nothing to commit") {
			return err
		}
	}

	// Push (force-with-lease is safe for artifact branches)
	cmd = exec.Command("git", "push", "origin", branchName, "--force-with-lease")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branchName, err)
	}

	fmt.Printf("✅ Branch '%s' updated with only 'public/' directory\n", branchName)
	return nil
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
